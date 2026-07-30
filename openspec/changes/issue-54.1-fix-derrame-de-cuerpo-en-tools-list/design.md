# Acotar el cuerpo por item en los tools MCP de listado y búsqueda — Design

DOMAINSERV-161 · issue-54.1 · flow run `e62781f9-3244-48b8-a4af-96fd7498088e`

## Decisions

### ADR-161.1 — Preservar el naming PascalCase al proyectar

`Skill`, `Agent`, `Flow` y `SearchResult` no declaran tags json, así que sus keys JSON son los nombres de campo Go. Al proyectar se conserva ese naming: solo desaparece el campo del cuerpo y aparece su `*Len`.

Alternativas descartadas: normalizar a snake_case en el DTO (cambia TODAS las keys, convirtiendo un breaking acotado en uno ancho) y agregar tags json a los structs de dominio (los consumen callers fuera de MCP).

Tradeoff aceptado: se perpetúa la inconsistencia de naming. El ticket es sobre el derrame; mezclarle un rename agrega un vector de rotura que no comparte causa ni verificación con el problema real.

Patrón: DTO con shadowing por embedding, como `ticket_slim_dto.go:20-24`.

### ADR-161.2 — Snippet de 200 caracteres en la misma key

El cuerpo truncado viaja bajo el mismo nombre de key que hoy (`content` / `Snippet`), más un campo de longitud.

El número sale del único precedente medido y unánime: 11 sitios ya truncan a 200. Los 3000/4000 de `project_index_tools.go` son cuerpos de documento, no previews de listado.

No se renombra a `snippet` porque el hook `domain-user-prompt.sh:204-227` lee `r.get("content")` y ya trunca a 160 por su cuenta: con 200 bajo la misma key, el consumidor más crítico no se entera. Lo que lo rompería en silencio es el rename, no el truncado.

Patrón: reuso de `truncate(s, n)` de `project_index_tools.go:298`, que agrega marcador en vez de cortar seco.

### ADR-161.3 — Guard runtime y lint estático son complementarios

El guard de bytes va en `resilience.go:340`, justo después de `execWithRetry`: es el único punto donde el resultado está disponible antes de cachear, invalidar y reportar métricas. `Wrap` es el chokepoint real (165 `wrap.Wrap(` contra 165 `Handler:`), no `toolResultJSON`, al que 17 retornos esquivan.

El guard emite un envelope JSON válido con `truncated=true`. Un corte por substring partiría el JSON, el hook `SessionStart` fallaría el `json.loads`, el token caería a `skpol=degraded` y el agente re-llamaría las tools pesadas: la regresión de 28.206 tokens que eliminó DOMAINSERV-177. Un guard mal hecho acá empeora lo que el ticket resuelve.

Tradeoff: dos mecanismos en vez de uno. Cubren fallas distintas — el lint evita que se escriba mal, el wrapper evita que se escape lo escrito.

Nota de seguridad atribuible a este cambio: el guard introduce un punto donde contenido controlable por el usuario determina dónde se corta el payload. Mitigación incorporada al diseño: el truncado opera sobre el valor antes de serializar y se re-serializa como envelope, en vez de cortar la cadena JSON final.

### ADR-161.4 — El detalle se pide con fan-out, no con opt-in de cuerpo completo

`sdd-review` hace fan-out de `domain_policy_get` por slug; `context-preservation` hace fan-out de `domain_mem_get_observation` por id. No se agrega `full_content=true`.

Un opt-in de "devolveme todo" es una escotilla que cualquier caller puede prender por comodidad, reviviendo el derrame. El precedente `include_globals` es distinto: filtra filas, no infla el cuerpo por fila.

Consecuencia crítica: `sdd_review.go:79` no incluye `domain_policy_get` en `RequiredToolCalls`, y `Validate()` (`:88-96`) solo bloquea con `verdict == "violations_found"`, sin verificar `policies_checked > 0`. Sin fan-out, el gate evaluaría contra cuerpos vacíos y emitiría `compliant`. Por eso fan-out y proyección van en el mismo commit.

Patrón: list-then-fetch.

### ADR-161.5 — El linter se clona, no se extiende

Binario nuevo que reusa el esqueleto de `cmd/response-shape-lint` (parseo `go/ast`, tipo `Violation`, exit codes), con job propio de CI.

No se extiende porque su detección de handlers está acoplada a la firma `(w http.ResponseWriter, r *http.Request)` (`main.go:184-219`) y su `runShapeChecks` (`:43`) depende de un archivo de rutas y de snapshots que no existen en `internal/mcp/server`.

Se descarta el baseline al estilo `.size-lint-baseline`: los 17 sitios son mecánicamente normalizables, así que el guard arranca en cero violaciones.

## Data Flow

```
handler → resultado
   ↓
proyección por tool (DTO slim: cuerpo fuera, *Len dentro; snippet 200 en búsquedas)
   ↓
toolResultJSON (helper canónico, verificado por el lint AST en CI)
   ↓
ResilientWrapper.Wrap → execWithRetry → [GUARD DE BYTES] → cache → métricas
   ↓
consumidor
   ├─ hook: lee content (≤200), ya truncaba a 160 → sin cambios
   ├─ sdd-review: slugs → fan-out policy_get → cuerpo real
   └─ context-preservation: id → fan-out mem_get_observation → resumen completo
```

## TDD Plan

Orden: primero el molde compartido, porque las 9 proyecciones heredan su mecanismo de test. Después las tools sin consumidor delicado, luego el guard y el lint, y al final el gate, que es lo único cuya verificación exige ejecución.

| # | Test | Qué fija | Sabotaje |
|---|---|---|---|
| 1 | `TestSkillSlim_Proyectar_ContentLargo_OmiteCuerpoYExponeLen` | Ausencia de la clave del cuerpo, ausencia de la subcadena, y `ContentLen` con valor exacto | Quitar `omitempty` del campo shadowed: la clave reaparece en el JSON y el test debe fallar |
| 2 | `TestMemSearch_Handler_ContentLargo_TruncaA200YReportaLen` | `content` con 200 caracteres exactos y `content_len` con el largo real | Cambiar el 200 por 201 en la llamada a `truncate`: la aserción de longitud debe romper |
| 3 | `TestMemContext_Handler_MismoShapeQueMemSearch` | Que las dos tools no diverjan en el shape del mismo objeto | Devolver `content` sin truncar solo en `mem_context`: el test comparativo debe fallar |
| 4 | `TestResilientWrapper_Wrap_PayloadSobreLimite_DevuelveJSONValidoConTruncated` | Que la salida truncada siga siendo parseable y traiga `truncated=true` | Reemplazar el envelope por un corte crudo de la cadena serializada: el `json.Unmarshal` del test debe fallar |
| 5 | `TestResilientWrapper_Wrap_EscapeUnicodeEnElBorde_NoRompeElParseo` | El caso borde de seguridad del ADR-161.3 | Mover el truncado a después de serializar: el escape queda partido y el parseo falla |
| 6 | `TestMCPResultLint_CallToolResultManual_ReportaViolacion` | Que el linter detecte un resultado armado a mano | Comentar la rama del AST que matchea el literal del struct: el linter devuelve 0 violaciones sobre el fixture y el test falla |
| 7 | `TestMCPResultLint_RepoActual_CeroViolaciones` | Que los 17 sitios quedaron normalizados y no hay deuda tolerada | Revertir uno solo de los 17 a la forma manual: el conteo deja de ser 0 |
| 8 | `TestSDDReview_Validate_VerdictViolations_BloqueaElAvance` | Que el gate sigue bloqueando | Cambiar la comparación de `verdict` para que acepte cualquier valor no vacío: el diff en violación pasaría a archive |
| 9 | `TestAgentTemplatesSeeder_PromptEditado_ExigeBumpDeVersion` | Que una edición del prompt sin bump no pueda pasar inadvertida | Editar el prompt dejando la constante en 20: el test debe detectar la divergencia |

Verificación que NO es un test unitario y se hace por ejecución, según exige el ticket: correr la fase `sdd-review` con el shape nuevo sobre un diff que viola una policy conocida, y comprobar que el verdict sale `violations_found`. Hoy, con slugs sin cuerpo, devuelve `compliant` y pasa a archive.

Método aplicado a cada sabotaje: conteo de ocurrencias antes y después con la ventana acotada al fragmento que gobierna el comportamiento, porque en este repo hay 5 casos registrados de falso positivo por subcadena. En un guard, un error de setup es un fallo, nunca un skip.

## Risk Mitigation

| Riesgo | Mitigación en el diseño |
|---|---|
| Truncado que produce JSON inválido y degrada el bootstrap | Envelope válido con `truncated=true`; tests 4 y 5 |
| El gate aprueba contra cuerpos vacíos | Fan-out de `policy_get` en `RequiredToolCalls`; test 8 más verificación por ejecución |
| Bump del seeder olvidado, prompt viejo gobernando en prod | Bump en el mismo commit; test 9 |
| El hook se rompe y el CI no avisa por el filtro de paths | Hook y server en el mismo commit; ambas suites a mano |
| Shadowing frágil ante un rename en el struct de origen | Test 1 verifica ausencia de clave Y de subcadena, no solo el `*Len` |
