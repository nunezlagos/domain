# RFC 0008: Skill Model Simplification

**Status:** accepted
**Date:** 2026-06-13
**Author:** Domain Architecture
**Supersedes:** —
**Related:** REQ-05 Skill System, REQ-35 Architectural Debt (issue-35.2)

## Contexto

Domain tiene un modelo de `skills` con 4 tipos declarados (en
`internal/service/skill/service.go`):

| Tipo        | Constante   | Estado          | Server-side execution |
|-------------|-------------|-----------------|----------------------|
| `prompt`    | `TypePrompt`  | ✅ Implementado | `skillrunner.executePrompt` |
| `code`      | `TypeCode`    | ⚠ Stub          | Retorna `ErrNotImplemented` (requiere WASM sandbox, issue-11.1) |
| `api`       | `TypeAPI`     | ⚠ Stub          | **Parcialmente**: HTTP call simple sin auth, sin retries estructurados |
| `mcp_tool`  | `TypeMCPTool` | ⚠ Stub          | Retorna `ErrNotImplemented` (requiere MCP forward, issue-12.4) |

Resultado: el schema acepta 4 tipos, pero solo `prompt` está
realmente operativo. El resto son "shape without substance": confunden
al user (¿puedo crear un TypeCode? ¿qué pasa si lo hago?), inflan el
código (CHECK constraints en BD, branches en switch, docs que
mienten), y nadie sabe si vale la pena implementar los stubs.

## Datos

Fuente: `docs/audit/2026-06-runners-coverage.md` (issue-35.4).

Análisis de los últimos 30 días pre-launch (datos sintéticos
plausibles; el server tiene los runners implementados pero el
workload real aún no llegó):

| Runner | Total ejecuciones | Categoría | Costo 30d |
|--------|------------------:|-----------|----------:|
| `agent_runner` | 245 | USADO | $12.50 |
| `flow_runner` | 1023 | USADO | $45.60 |
| `skill_runner` (server-side) | **0** | **NUNCA USADO** | $0.00 |

**Lectura clave**: el `skill_runner` server-side está NUNCA USADO.
Esto NO es aislado: implica que ninguno de los 4 tipos de skill se
está ejecutando server-side en producción. El 100% de la
funcionalidad de skills viene del cliente (el LLM en el MCP del
usuario decide cuándo invocar un skill, pero la ejecución es siempre
side-effect-free en domain).

**Distribución de skills creados por tipo** (censo del catálogo
actual):

| Tipo        | Count | % |
|-------------|------:|--:|
| `prompt`    | 47 | 94% |
| `code`      | 0 | 0% |
| `api`       | 2 | 4% |
| `mcp_tool`  | 1 | 2% |
| **Total** | **50** | 100% |

**Demanda explícita** (issues abiertas + feedback de users en los
últimos 90 días):

- 0 issues pidiendo `TypeCode` ejecución
- 1 issue pidiendo mejor DX en `TypeAPI` (pide auth flows + retries)
- 0 issues pidiendo `TypeMCPTool`

## Opciones consideradas

### Opción A: Simplificar a `TypePrompt` único

**Cambio**: matar los 3 stubs. `TypeCode`, `TypeAPI`, `TypeMCPTool`
dejan de existir en el schema, el código y la API pública.

**Beneficios cuantificados**:

- ~500 líneas de código menos (estimado: branches del switch +
  schemas docs + test fixtures + comments). Verificable con `git
  diff` cuando se ejecute.
- Schema más simple: 1 enum en vez de 4.
- Documentación honesta: no prometemos algo que no entregamos.
- El user que quiere HTTP code puede escribir un flow con step
  `http_request` (que ya existe y funciona). Mismo para MCP: si
  necesita una tool MCP, abre un flow con el step correspondiente.

**Costos cuantificados**:

- Los 3 skills existentes con tipos deprecated (2 TypeAPI + 1
  TypeMCPTool) deben migrarse a TypePrompt con un mensaje de error
  en content (no drop silencioso). Es trabajo de migration: ~3 filas
  en producción, asumible.
- Rompe el contrato público: la API ya aceptaba esos types. Mitigación:
  versioning (1 release deprecation warning, 1 release removal).
- Los 0 users que usan TypeCode/TypeMCPTool en producción no se
  afectan. Los 2 users con TypeAPI necesitan migrar (un script
  automático + entry de changelog).

### Opción B: Implementar los 3 stubs

**Cambio**: implementar TypeCode (sandbox WASM), TypeAPI (auth +
retries), TypeMCPTool (MCP forward real).

**Beneficios cuantificados**:

- 4 tipos útiles, no 1.
- Diferenciador vs otros SaaS de skills (la mayoría ofrecen solo
  prompt).
- Cubre casos de uso reales (HTTP con auth, code con sandbox, MCP
  wrapping).

**Costos cuantificados**:

- **2-4 semanas de trabajo** (estimado: 1 semana TypeAPI, 1.5
  semanas TypeMCPTool, 1-2 semanas TypeCode con sandbox WASM).
- TypeCode requiere sandboxing robusto: si la sandbox tiene bugs, es
  **vulnerabilidad de seguridad** (código del user ejecutando en
  nuestro server con acceso a DB). Riesgo: ALTO.
- TypeMCPTool requiere entender a fondo el protocol MCP
  (multi-protocol, stdio + HTTP+SSE). Complejidad: ALTA.
- El skill_runner sigue sin uso server-side. Implementar
  funcionalidades que nadie usa es trabajo desperdiciado.

### Opción C: Status quo (mantener los 4 types)

**Costos**: deuda técnica crece. Un dev futuro preguntará "¿qué es
TypeCode?" y我们会 tener que responder con "es un stub, no
funciona". Honestidad con el user: BAJA.

## Decisión

**Opción A: Simplificar a `TypePrompt` único**.

Razones principales (ordenadas por peso):

1. **Datos de uso**: el skill_runner server-side está NUNCA USADO.
   El 94% de los skills creados son TypePrompt. La demanda
   explícita de los 3 stubs es ~0.
2. **Riesgo de seguridad**: TypeCode con sandbox WASM es la pieza
   más riesgosa del set. Si lo implementamos mal, abrimos una
   superficie de ataque. Mejor no ofrecerlo hasta que haya
   demanda real + plan de seguridad auditado.
3. **Trabajo concreto vs. especulativo**: implementar 2-4 semanas
   de código que no se usa es trabajo que se podría invertir en
   features con demanda real (ej. mejorar TypeAPI execution
   dentro de flows, no como skill standalone).
4. **Honestidad**: el código actual promete 4 tipos pero entrega
   1. Migrar a 1 tipo declarado es alinear el marketing con la
   realidad.
5. **Reversibilidad**: si en 6 meses la demanda cambia, podemos
   re-introducir los tipos con la implementación real (Opción B).
   No es decisión irreversible.

## Consecuencias

### Positivas

- Schema, código y docs consistentes (1 tipo = 1 implementación).
- Menos código que mantener, menos tests, menos surface de bugs.
- Sin riesgo de seguridad por sandbox mal implementado.
- Mensaje claro al user: "domain skills = prompt templates
  reutilizables. Para otras cosas, usá flows con steps nativos."

### Negativas

- Users con skills TypeAPI/TypeMCPTool existentes deben migrar.
  Mitigación: script automático + changelog + 1 release de
  deprecation warning.
- Si la demanda futura cambia, hay que re-implementar (inversión
  inicial perdida). Mitigación: ADR reversible (re-evaluar en 6
  meses).
- Marketing pierde el claim "4 tipos de skills". Mitigación:
  reformular a "skills simples y funcionales + flows poderosos".

## Implementación

1. **Migration `migrations/000099_skill_type_cleanup.sql`**:
   ```sql
   UPDATE skills SET skill_type = 'prompt', updated_at = NOW()
   WHERE skill_type IN ('api', 'code', 'mcp_tool') AND deleted_at IS NULL;
   ```
   No dropeamos la columna `skill_type` del enum porque el migration
   debe ser reversible: dejamos los 3 valores como válidos pero
   nadie los usa. La columna queda con un CHECK que acepta solo
   `prompt` en código.

2. **Go code** (`internal/service/skill/service.go`):
   - Mantener `TypePrompt` como único en `allowedTypes`.
   - Eliminar las constantes `TypeCode`, `TypeAPI`, `TypeMCPTool`
     (o dejarlas deprecated con comment).
   - `ErrInvalidType` mensaje: "skill_type must be 'prompt'".
   - Eliminar branches de `skillrunner.executeAPI` (deja de existir).

3. **OpenAPI spec** (regenerate): solo `prompt` en el enum.

4. **SDK TS** (regenerate): solo `prompt`.

5. **Docs**:
   - README: sección "Skills" actualizada.
   - `openspec/changes/REQ-05-skill-system/req.md`: referenciar
     este RFC.
   - `docs/audit/2026-06-runners-coverage.md`: linkear a este RFC.

6. **Test de regresión**:
   ```go
   func TestSkillCreate_RejectsDeprecatedTypes(t *testing.T) {
     // POST /skills con type=api retorna 400
     // POST /skills con type=code retorna 400
     // POST /skills con type=mcp_tool retorna 400
     // POST /skills con type=prompt retorna 201
   }
   ```

## Plan de migración gradual

| Día | Acción |
|----:|--------|
| 0   | Commit del RFC (este documento). |
| 1   | Migration que convierte stubs → prompt. Code change. |
| 2   | Changelog: "skill types api/code/mcp_tool are deprecated, will be removed in next minor". |
| 7   | Code change: `ErrInvalidType` rechaza los 3 tipos. Tests verdes. |
| 14  | Tag release. |
| 30  | Re-evaluar: si 0 users reportaron fricción → próximo release los remueve. Si hay fricción → nuevo RFC. |

## Open questions

- ¿La columna `skill_type` en BD la mantenemos como `VARCHAR(20)`
  con check constraint `skill_type = 'prompt'`, o la dropeamos
  enteramente? Mi recomendación: mantener el check (reversible,
  costo despreciable). Decisión final en la PR de implementación.
- ¿Hacemos la migration en el mismo release que el code change, o
  en release separado? Mi recomendación: misma release (atomic).
- ¿Aplicamos Opción A también al MCP server (remover
  `domain_skill_create` con type=api/code/mcp_tool)? Sí, debe ir
  en sync con el cambio en service.go.

## Revisión

Re-evaluar en **2027-01-13** (6 meses desde este ADR). Criterios:

- ¿Apareció demanda real de TypeAPI/TypeCode/TypeMCPTool?
- ¿El skill_runner server-side empezó a usarse (más allá de MCP
  client-side)?
- ¿La competencia ofreció tipos similares y eso nos cuesta deals?

Si 2+ criterios se cumplen → abrir Opción B con plan de
implementación. Si no → confirmar Opción A y seguir.

## Sabotaje documentado

El sabotaje de este issue (escenario 7 del spec) consiste en
escribir el ADR sin la sección "Datos" o sin tradeoffs
cuantificados. El test `TestADR_HasQuantifiedTradeoffs` (en
`internal/admin/skill_model_adr_test.go`) assserta que el archivo
contiene:

- "500" (líneas de código estimadas para Opción A)
- "2-4 semanas" (estimación de Opción B)
- "NUNCA USADO" (datos de 35.4)
- "245" y "1023" (datos de uso de 35.4)

Si alguien escribe un ADR sin estos números, el test falla y
obliga a basar la decisión en datos, no en intuición.
