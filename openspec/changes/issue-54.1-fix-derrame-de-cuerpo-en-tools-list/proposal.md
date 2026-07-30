# Acotar el cuerpo por item en los tools MCP de listado y búsqueda

DOMAINSERV-161 · flow run `e62781f9-3244-48b8-a4af-96fd7498088e`

## Why

Ocho tools `domain_*` de listado más `domain_mem_context` serializan el cuerpo completo de cada item. `domain_mem_context` es el peor caso: el hook `SessionStart` lo inyecta en cada sesión con el `content` entero de hasta 20 observaciones, lo que lo convierte en el mayor emisor de cuerpo por sesión de todo el sistema.

El shape de respuesta es un contrato. Cuatro de los ocho consumidores río abajo fallan en silencio si el cuerpo desaparece: no hay error ni log, solo comportamiento degradado indistinguible del éxito. Uno de ellos es el gate `sdd-review`, que pasaría a aprobar todo sin avisar.

## Scope

### Entra

- Proyección de 8 tools de listado/búsqueda + `domain_mem_context`.
- Guard de bytes en `ResilientWrapper.Wrap` con envelope JSON válido y `truncated=true`.
- Linter AST nuevo que detecta `&mcp.CallToolResult{}` armados a mano en `internal/mcp/server`.
- Normalización de los 17 sitios manuales existentes a `toolResultJSON`.
- Rediseño del gate `sdd-review` a fan-out de `domain_policy_get`, con bump de `agentTemplatesSeedVersion` 20→21.
- Actualización de la policy `context-preservation` para re-hidratar con `domain_mem_get_observation`.
- Actualización del hook `install-user/hooks/domain-user-prompt.sh` en el mismo commit que el server.

### No entra

| Item | Razón |
|---|---|
| Normalizar keys a snake_case en `Skill`, `Agent`, `Flow`, `SearchResult` | Cambiaría todas las keys, no solo el cuerpo: breaking más ancho que el declarado |
| Tags json en los structs de dominio de `internal/service/` | Esos structs los consumen callers fuera de MCP; el blast radius se va del módulo |
| `domain_ticket_list` | Ya resuelta por DOMAINSERV-177/178. Se verifica, no se reimplementa |
| Refactorizar `cmd/response-shape-lint` para cubrir ambos mundos | Su AST-scan está acoplado a la firma `(w http.ResponseWriter, r *http.Request)` |
| Paginación o cursores | Ortogonal al derrame |
| Logging en el pipeline de hooks | Pre-existing; es la razón de que estas fallas sean invisibles, pero no se aborda acá |

## Approach

1. Clonar el molde `ticketSlim` (embedding + shadowing con `campoOmitido` + campo `*Len`) preservando el naming PascalCase de los structs que hoy no tienen tags json.
2. En las tres tools de búsqueda, truncar a 200 caracteres reusando el helper `truncate` de nivel de paquete, manteniendo el nombre de la key que hoy lleva el cuerpo.
3. Insertar el guard de bytes justo después de `execWithRetry`, de modo que también acote lo que entra al cache.
4. Clonar el esqueleto del linter existente (parseo AST, tipo `Violation`, exit codes) en un binario nuevo con su propio job de CI.
5. Reescribir el prompt seedeado de `sdd-review` para fan-out por slug, con el bump del seeder en el mismo commit.

## Risks

| Riesgo | Mitigación |
|---|---|
| Un truncado por substring produce JSON inválido y degrada el bootstrap a `skpol=degraded`, reviviendo la regresión de 28k tokens de DOMAINSERV-177 | El guard emite un envelope JSON válido; test que parsea la salida truncada |
| El gate `sdd-review` queda evaluando contra cuerpos vacíos y aprueba todo en silencio | Fan-out de `policy_get` + verificación por EJECUCIÓN de que un diff en violación sale rojo |
| El bump del seeder se olvida y el prompt viejo sigue gobernando en producción | Bump en el mismo commit; el modo de falla es indistinguible del éxito, así que se verifica explícitamente |
| El hook de install-user se rompe y el CI no avisa, porque filtra por `services/domain-mcp/**` | Hook y server en el mismo commit; ambas suites corridas a mano |
| El shadowing por embedding es frágil ante un rename del campo en el struct de origen | Test que verifica ausencia de la clave y del contenido, no solo presencia del `*Len` |

## Testing

- Por tool: un test que marshalea la respuesta y verifica ausencia de la clave del cuerpo, ausencia de la subcadena del contenido, y presencia del `*Len` con el valor exacto. Es el mecanismo de `ticket_slim_dto_test.go`.
- Guard de bytes: test que fuerza un payload sobre el límite y parsea la salida, más el caso del escape `\uXXXX` cortado en el borde.
- Gate `sdd-review`: verificación por ejecución con un diff en violación conocida; el verdict debe salir `violations_found`.
- Hook: ejercitar el parseo real contra el shape nuevo. Hoy no hay ningún test que lo cubra.
- Suites de ambos módulos corridas a mano, porque el filtro de paths del CI no dispara la de `install-user`.
