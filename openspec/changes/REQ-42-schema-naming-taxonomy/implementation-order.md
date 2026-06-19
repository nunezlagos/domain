# Implementation Order — REQ-42 schema-naming-taxonomy

> **Orden de implementación propuesto** para las 11 HUs hijas de REQ-42, organizado en **6 olas secuenciales**. El criterio rector: **el catálogo primero** (contrato), **los drops antes que los renames** (reducir superficie), y **el lint al final** (enforcing a futuro, sin bloquear las propias migraciones del REQ).

## Principios del orden

1. **El catálogo es el contrato y va primero.** `table_catalog` (42.1) es la única fuente de verdad que gobierna grupo/label/sort_order. Las HUs de rename actualizan el catálogo en su misma migración (`UPDATE table_catalog SET table_name = ...`); sin el catálogo creado, no tienen contra qué validar. Por eso 42.1 abre todo y NO renombra ni dropea nada.
2. **Drops antes que renames: reducir la superficie.** Cada tabla que se dropea (42.2 billing, 42.3 legacy/infra) es una tabla que NO hay que renombrar, ni clasificar en el catálogo, ni mapear en el admin, ni cubrir con tests de rename. Sacarlas temprano reduce el trabajo de las olas siguientes y elimina ruido del grafo de FKs (ej. limpiar las FKs entrantes de `sessions` antes de tocar `captured_prompts`/`verifications`).
3. **La tabla nueva va después de los drops y antes de los renames.** `agent_run_prompts` (42.4) ya nace con el prefijo correcto; se crea sobre un schema ya adelgazado (post-drops) y antes de la oleada de renames para no mezclarse con ellos.
4. **Renames agrupados por dominio, una migración atómica por dominio.** Cada migración renombra un dominio completo en una sola transacción `BEGIN/COMMIT`, arrastrando índices/sequences/constraints/RLS (patrón 000146). Se separan por dominio para acotar el blast radius y los touchpoints Go de cada deploy.
5. **Angular después de que el schema esté estable.** El explorador `/database` (42.10) deriva los grupos del prefijo REAL de las tablas; si corre antes de los renames, agruparía nombres viejos. Va después de la ola de renames.
6. **El lint al final, en enforcing.** Si `dbconvlint` rechazara nombres sin prefijo ANTES de los renames, bloquearía las propias migraciones de REQ-42 (que parten de nombres legacy). Se activa al final para enforce a partir de ahí, una vez que todo el schema ya cumple la convención.

## Olas

### Ola 1 — Taxonomía + catálogo (fundación)

| HU | Migración | Tipo | Por qué acá |
|----|-----------|------|-------------|
| **42.1** taxonomia-y-catalogo | 000147 | feature (DDL aditivo) | Crea y siembra `table_catalog` con los nombres ACTUALES (pre-rename). Es el contrato que todas las olas siguientes respetan. No toca el schema vivo. |

### Ola 2 — Drops (reducir superficie)

| HU | Migración | Tipo | Por qué acá |
|----|-----------|------|-------------|
| **42.2** drop-billing-costos | 000148 | drop + refactor Go | Elimina 5 tablas de billing/costos. Modelo free total → no se renombran, se borran. Menos tablas que clasificar/renombrar después. |
| **42.3** drop-legacy-infra | 000149 | drop + refactor Go | Elimina 8 tablas legacy/infra. Limpia las FKs entrantes de `sessions` (`captured_prompts.session_id`, `verifications.session_id`) ANTES de que esas dos tablas se renombren en olas posteriores. |

> 42.2 y 42.3 son independientes entre sí (dominios disjuntos) y pueden paralelizarse, pero ambas deben terminar ANTES de la ola de renames.

### Ola 3 — Tabla nueva

| HU | Migración | Tipo | Por qué acá |
|----|-----------|------|-------------|
| **42.4** tabla-agent-run-prompts | 000150 | feature (DDL aditivo) | Crea `agent_run_prompts` ya con prefijo correcto, sobre un schema adelgazado. Aislada de los renames para no mezclar DDL aditivo con DDL de rename. |

### Ola 4 — Renames por dominio (RENAME directo single-org, atómico)

| HU | Migración | Dominio | Por qué acá |
|----|-----------|---------|-------------|
| **42.5** rename-sdd-tdd | 000151 | SDD/TDD + capa issue | El dominio con más touchpoints Go (pipeline SDD completo). `requirements/proposals/designs`→`sdd_*`, `verifications/verification_results/sabotage_records`→`tdd_*`, `tasks/code_references/intake_payloads`→`issue_*`. |
| **42.6** rename-issues | 000152 | issue (gherkin) | `gherkin_scenarios`→`issue_gherkin_scenarios`. Separada de 42.5 porque toca un set acotado de queries (servicio issue) y permite un deploy/test focalizado. |
| **42.7** rename-enrollment | 000153 | auth (enrollment) | `org_enrollment_tokens`→`enrollment_tokens`. Saca el prefijo `org_` legacy. Rename de bajo riesgo, aislado. |
| **42.8** rename-auth-users | 000154 | auth | `otp_codes/api_keys/secrets/invitations`→`auth_*`. Arrastra RLS policies (`otp_codes`). `users/roles/user_roles` quedan canónicas (NO se renombran). Una sola migración atómica para el grupo credenciales. |
| **42.9** rename-resto | 000155 | varios | Cierre: `clients`→`project_clients`, `captured_prompts`→`prompt_captured`, `observations`→`knowledge_observations`, `outbound_webhook_*`→`webhook_outbound_*`, `selfhosted_*`→`runner_selfhosted_*`, `activity_log`→`audit_activity_log`, `imported_workflow_files`→`project_imported_workflow_files`. |

> Las migraciones 42.5→42.9 son secuenciales en numeración (000151→000155) pero los dominios son disjuntos; el orden interno de la ola es por volumen de touchpoints (el más pesado primero, 42.5) para amortiguar el riesgo temprano. Cada una va en su PROPIO deploy junto con su código Go.

### Ola 5 — Angular (consumir el schema renombrado)

| HU | Migración | Tipo | Por qué acá |
|----|-----------|------|-------------|
| **42.10** angular-grouping-database | — | frontend | Reescribe `/database` para agrupar por prefijo REAL. Debe correr DESPUÉS de los renames, si no agruparía nombres viejos. Oculta `schema_migrations`, muestra `seed_versions` como "Seeders corridos". |

### Ola 6 — Lint (enforcing a futuro)

| HU | Migración | Tipo | Por qué acá |
|----|-----------|------|-------------|
| **42.11** lint-enforce-prefix | — | tooling | `dbconvlint` rechaza toda `CREATE TABLE` sin prefijo válido. Va último: si se activara antes, bloquearía las migraciones de REQ-42 que parten de nombres legacy. Activado el final, el schema ya cumple y la regla queda enforcing para el futuro. |

## Asignación de migraciones (consolidado)

| Migración | HU | Operación |
|-----------|----|-----------|
| 000147 | 42.1 | CREATE + seed `table_catalog` |
| 000148 | 42.2 | DROP billing/costos (5 tablas) |
| 000149 | 42.3 | DROP legacy/infra (8 tablas) + limpiar FKs de `sessions` |
| 000150 | 42.4 | CREATE `agent_run_prompts` |
| 000151 | 42.5 | RENAME SDD/TDD + capa issue |
| 000152 | 42.6 | RENAME `gherkin_scenarios`→`issue_gherkin_scenarios` |
| 000153 | 42.7 | RENAME `org_enrollment_tokens`→`enrollment_tokens` |
| 000154 | 42.8 | RENAME grupo AUTH (users/roles/user_roles canónicas) |
| 000155 | 42.9 | RENAME resto |
| — | 42.10 | (frontend Angular, sin migración) |
| — | 42.11 | (tooling lint, sin migración) |

> Última migración aplicada antes de REQ-42: **000146** (`org_flow_config`→`flow_config`). REQ-42 ocupa 000147–000155 (9 migraciones). 42.10 y 42.11 no crean migración.

## Riesgos del plan

| Riesgo | Mitigación |
|---|---|
| Una migración de rename corre sin su código Go en el mismo deploy → `relation ... does not exist` | Regla explícita en cada `tasks.md`: migración + touchpoints Go en el MISMO deploy. Grep final de nombres legacy antes de commitear. |
| El catálogo queda desincronizado si un rename corre antes de actualizarlo | Cada HU de rename incluye el `UPDATE table_catalog SET table_name = ...` en su misma migración. |
| Rename de `verifications`→`tdd_verifications` falla por RLS/`organization_id` residual (migración 000111) | Verificación previa bloqueante en 42.5: confirmar que 000132 ya removió RLS + `organization_id` antes de renombrar constraints/policy. |
| DROP de `sessions` deja FKs colgando en `captured_prompts`/`verifications` | 42.3 limpia esas columnas en la misma migración, ANTES de que esas tablas se renombren en olas posteriores. |
| Tests verdes "por casualidad" (no detectan un rename incompleto) | Cada HU de rename tiene sección de **sabotaje**: comentar UN `RENAME CONSTRAINT` o dejar UNA query legacy → el test DEBE caer; si pasa en verde, el test es falso positivo y se arregla antes de seguir. |
| Activar el lint antes de tiempo bloquea las migraciones de REQ-42 | 42.11 es la ÚLTIMA ola, cuando el schema ya cumple la convención. |
| `tests/e2e/schema_audit_test.go` ya está desincronizado (tablas inexistentes) | Se actualiza por coherencia con un `// TODO: schema audit desincronizado (REQ-42)`; la limpieza completa es de otra HU. |

## Criterios de "done" globales para el REQ-42

- [ ] `table_catalog` creada, sembrada y consumida por el admin
- [ ] 13 tablas dropeadas (5 billing + 8 legacy/infra) y su código Go removido/refactorizado
- [ ] `agent_run_prompts` creada y poblándose por iteración de `agent_run`
- [ ] Todas las tablas conservadas con prefijo de grupo (salvo nombre canónico `users`/`issues` si se confirma, y `schema_migrations` interno)
- [ ] Pipeline SDD/TDD verde de punta a punta con nombres nuevos
- [ ] `/database` agrupa por funcionalidad; `schema_migrations` oculta; `seed_versions` visible
- [ ] `dbconvlint` rechaza `CREATE TABLE` sin prefijo válido
- [ ] Tests de sabotaje pasan en cada HU de rename (romper → cae → restaurar → verde)
- [ ] `go vet`, `go build`, `go test`, `squawk` verdes
- [ ] Grep final de nombres legacy en `internal/`, `cmd/`, `tests/` → 0 resultados
- [ ] Cada migración tiene su `down` reversible y atómico
- [ ] Commits en rama `services` (Conventional Commits, español, SIN Co-Authored-By); NO git push (repo local-only)
