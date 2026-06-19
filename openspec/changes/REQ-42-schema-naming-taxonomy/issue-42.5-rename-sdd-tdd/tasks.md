# Tasks: issue-42.5-rename-sdd-tdd

> **Regla de deploy:** la migración 000151 y TODOS los cambios de código de abajo van en el **MISMO deploy**. Aplicar la migración con el binario viejo (o al revés) rompe el pipeline con `relation ... does not exist`.

## Verificación previa (bloqueante)

- [ ] Confirmar que 000146 es la última migración aplicada y que 000151 es el número asignado a esta HU (147-150 reservados a 42.1-42.4)
- [ ] Releer la introspección y confirmar los nombres REALES de índices/constraints legacy (`proposals_hu_id_version_key`, `designs_hu_id_fkey`, `code_references_hu_id_*`, `intake_payloads_committed_hu_id_fkey`, `gherkin_hu_id_idx`, `tasks_hu_id_fkey`, `verifications_org_*`)
- [ ] Confirmar que las 11 tablas no tienen sequence (10 con UUID PK; `sabotage_records` con `id` no-serial)
- [ ] Confirmar que `verifications` ya NO tiene `organization_id` ni policy RLS (000132)
- [ ] Confirmar los DOS triggers de `intake_payloads` (`trg_set_updated_at` genérico + `set_updated_at_intake_payloads` legacy)
- [ ] Confirmar que `selfhosted_tasks` NO está en alcance (otra tabla)

## Migración (000151)

- [ ] `000151_rename_sdd_tdd_issue_taxonomy.up.sql` — header completo (migration/author/issue/description/breaking/estimated_duration), `BEGIN/COMMIT`, `ALTER ... IF EXISTS`, snake_case plural; incluye el rename `sabotage_records → tdd_sabotage_records` (bloque 10)
- [ ] `000151_rename_sdd_tdd_issue_taxonomy.down.sql` — inverso atómico, restaura nombres legacy `*_hu_id_*` + trigger `set_updated_at_intake_payloads` + `tdd_sabotage_records → sabotage_records`
- [ ] Verificar que NO hay `ALTER SEQUENCE` (10 con UUID PK + `sabotage_records` `id` no-serial)
- [ ] Verificar que el trigger genérico `trg_set_updated_at` NO se toca
- [ ] Verificar que NO se renombra ninguna RLS policy (no existen para estas tablas)
- [ ] `ALTER TABLE tdd_verifications NO FORCE ROW LEVEL SECURITY` presente
- [ ] Pasar `squawk` sobre up y down

## Backend — touchpoints `sdd_requirements` (ex `requirements`)

- [ ] `internal/service/requirement/service.go` — 8 queries SQL (INSERT ~120, SELECT 149/165/203/327/330, UPDATE 260/298/302) → `sdd_requirements`
- [ ] `internal/service/issue/service.go` — `SELECT id FROM requirements` (~117) y `LEFT JOIN requirements r` (~234) → `sdd_requirements`
- [ ] `internal/service/traceability/service.go` — 6 referencias (líneas 126/222/272/310/340/397) → `sdd_requirements`
- [ ] `internal/service/wizardplan/sources/issue_dedup.go` — `JOIN requirements r ON r.id = us.req_id` (~77) → `sdd_requirements`
- [ ] `internal/service/attachment/service.go` — `NOT EXISTS (SELECT 1 FROM requirements ...)` (~199) → `sdd_requirements`
- [ ] `internal/runner/intake/worker.go` — referencia a `requirements` en el commit del intake → `sdd_requirements` (verificar)
- [ ] `internal/service/projectdetect/detect.go` — **confirmar primero** si es SQL o texto; tocar SOLO si es literal SQL
- [ ] `internal/api/handler/requirement.go` — confirmar si embebe SQL o sólo llama al service
- [ ] `internal/agentprotocol/protocol.go` — **confirmar primero** (probable constante/etiqueta de dominio, NO SQL); NO tocar si no es SQL

## Backend — touchpoints `sdd_proposals` (ex `proposals`)

- [ ] `internal/service/spec/service.go` — `SELECT MAX(version) FROM proposals` (~95), INSERT (~100), SELECT (126/142/157/170), UPDATE (~204) → `sdd_proposals`
- [ ] `internal/service/traceability/service.go` — 4 referencias (157/244/297/342) → `sdd_proposals`
- [ ] `internal/mcp/server/proposals_tools.go` — SQL embebido → `sdd_proposals` (el nombre del archivo NO cambia)
- [ ] `internal/service/orchestrator/phases/sdd_propose.go` — verificar que no haya raw SQL a `proposals`
- [ ] `internal/service/orchestrator/analysis/service.go` — verificar referencias a `proposals`
- [ ] `internal/api/handler/spec.go` — verificar SQL embebido vs llamada a service
- [ ] `internal/api/handler/rest_new.go` — verificar referencia a `proposals`
- [ ] `internal/api/handler/api.go` — verificar (probable routing/labels, no SQL)

## Backend — touchpoints `sdd_designs` (ex `designs`)

- [ ] `internal/service/spec/service.go` — `SELECT MAX(version) FROM designs` (~235), INSERT (~244), SELECT (270/285), UPDATE (~301) → `sdd_designs`
- [ ] `internal/service/traceability/service.go` — 4 referencias (163/245/302/343) → `sdd_designs`
- [ ] `internal/service/orchestrator/analysis/service.go` — verificar referencias a `designs`
- [ ] `internal/api/handler/spec.go` — verificar SQL embebido vs service
- [ ] `internal/api/handler/api.go` — verificar (routing/labels)

## Backend — touchpoints `issue_gherkin_scenarios` (ex `gherkin_scenarios`)

- [ ] `internal/service/issue/service.go` — `SELECT MAX(position)` (~363), INSERT (368 y 462), DELETE (~381), SELECT (411/425) → `issue_gherkin_scenarios`

## Backend — touchpoints `issue_tasks` (ex `tasks`)

- [ ] `internal/service/task/service.go` — `SELECT MAX(position) FROM tasks` (~122), INSERT (~135), SELECT (156/169/204/274), UPDATE (~243), `SELECT status FROM tasks` (~290), `SELECT EXISTS(... FROM tasks)` (~317) → `issue_tasks`. **CUIDADO:** `tasks` también es identificador Go; tocar SOLO literales SQL.
- [ ] `internal/service/traceability/service.go` — `FROM tasks` (~172), `LEFT JOIN tasks t` (246/274/344), `SELECT issue_id FROM tasks` (~312) → `issue_tasks`

## Backend — touchpoints `issue_code_references` (ex `code_references`)

- [ ] `internal/service/traceability/service.go` — SELECT (~180), `FROM code_references cr` (~208), `LEFT JOIN LATERAL ... FROM code_references` (~247), INSERT (~372), DELETE (~386) → `issue_code_references`

## Backend — touchpoints `issue_intake_payloads` (ex `intake_payloads`)

- [ ] `internal/service/intake/service.go` — INSERT (~131), FROM (170/303), UPDATE (184/203/227/257/278) → `issue_intake_payloads`
- [ ] `internal/runner/intake/worker.go` — SQL a `intake_payloads` (y a `requirements`/`issues` en el commit del payload) → `issue_intake_payloads` / `sdd_requirements`
- [ ] `internal/service/attachment/service.go` — verificar referencia a `intake_payloads` (entity_type='intake' o similar)

## Backend — touchpoints `tdd_verifications` (ex `verifications`)

- [ ] `internal/mcp/server/verifications_tools.go` — INSERT (~137), `SELECT items FROM verifications` (185/241), UPDATE (213/278), FROM (~319) → `tdd_verifications`
- [ ] `internal/mcp/server/health_tools.go` — `SELECT COUNT(*) FROM verifications WHERE status IN (...)` (~94) → `tdd_verifications` (la clave del check `verifications_open` puede quedar igual)
- [ ] `internal/api/handler/rest_new.go` — `FROM verifications` (~413) → `tdd_verifications`
- [ ] `internal/api/handler/api.go` — verificar referencia a `verifications` (routing/labels)
- [ ] `cmd/domain/seed_demo.go` — `INSERT INTO verifications` (~552) y `SELECT 1 FROM verifications` (~556) → `tdd_verifications`

## Backend — touchpoints `tdd_verification_results` (ex `verification_results`)

- [ ] `internal/service/task/service.go` — `INSERT INTO verification_results` (~303) y `SELECT FROM verification_results WHERE task_id` (~368) → `tdd_verification_results`

## Backend — touchpoints `tdd_sabotage_records` (ex `sabotage_records`)

- [ ] `internal/service/task/service.go` — `INSERT INTO sabotage_records (...)` (~327) y `FROM sabotage_records WHERE task_id` (~342) → `tdd_sabotage_records`. **SOLO los literales SQL.**
- [ ] **NO tocar** `internal/service/orchestrator/phases/sdd_judge.go` (~48/63/65) ni `registry.go` (~115): ahí `"sabotage_records"` es una CLAVE de payload JSON del judge, NO el nombre de tabla.
- [ ] `tests/e2e/schema_audit_test.go` — `sabotage_records` en `expectedTables` (~48) → `tdd_sabotage_records`; su FK `{"sabotage_records","task_id","tasks"}` → `{"tdd_sabotage_records","task_id","issue_tasks"}`

## Tests e2e (schema audit — ya desactualizado, actualizar por coherencia)

- [ ] `tests/e2e/schema_audit_test.go` — `expectedTables` (líneas 47-52): `requirements`→`sdd_requirements`, `proposals`→`sdd_proposals`, `designs`→`sdd_designs`, `gherkin_scenarios`→`issue_gherkin_scenarios`, `tasks`→`issue_tasks`, `code_references`→`issue_code_references`, `intake_payloads`→`issue_intake_payloads`, `verification_results`→`tdd_verification_results`, `sabotage_records`→`tdd_sabotage_records` (y `verifications`→`tdd_verifications` si está listada)
- [ ] `schema_audit_test.go` FKs (líneas 188-194): `{"issues","req_id","requirements"}`→`...sdd_requirements`, `{"proposals",...}`→`{"sdd_proposals",...}`, `{"designs",...}`→`{"sdd_designs",...}`, `{"tasks",...}`→`{"issue_tasks",...}`, `{"intake_payloads",...}`→`{"issue_intake_payloads",...}`
- [ ] **NOTA:** este test ya referencia tablas inexistentes (`organizations`, `intake_attachments`, `custom_roles`, `event_log`). No es el foco de esta HU arreglarlas, pero dejar un comentario `// TODO: schema audit desincronizado (REQ-42)` para una HU de limpieza.

## Verificación de la migración (TDD)

- [ ] Test up: las 11 tablas nuevas existen (`to_regclass` IS NOT NULL) y las 11 viejas NO (IS NULL)
- [ ] Test FK por OID: `tdd_verification_results.task_id → issue_tasks`, `tdd_sabotage_records.task_id → issue_tasks` y `sdd_designs.proposal_id → sdd_proposals` (consultar `pg_constraint`)
- [ ] Test naming: 0 índices/constraints con substring `_hu_id_` en las tablas afectadas
- [ ] Test naming: 0 índices con substring `_org_` en `tdd_verifications`
- [ ] Test triggers: `issue_intake_payloads` tiene `trg_set_updated_at` + `set_updated_at_issue_intake_payloads`; NO tiene `set_updated_at_intake_payloads`
- [ ] Test rollback: up + down vuelve al estado original (nombres legacy + trigger legacy)

## Sabotaje (anti-falsos positivos)

> Objetivo: probar que los tests REALMENTE detectan un rename incompleto, no que pasan por casualidad.

1. **Sabotaje del constraint:** en `000151_...up.sql`, comentar UNA sola línea:
   `-- ALTER TABLE sdd_designs RENAME CONSTRAINT designs_proposal_id_fkey TO sdd_designs_proposal_id_fkey;`
   - Aplicar up.
   - **Esperado:** el test "0 constraints con `_hu_id_`/legacy sin re-prefijar" debe FALLAR porque `designs_proposal_id_fkey` queda con nombre viejo sobre `sdd_designs`.
   - Si el test PASA en verde con el rename comentado → el test es un falso positivo (no inspecciona `pg_constraint` de esa tabla). Arreglar el test ANTES de seguir.
2. **Sabotaje del código:** en `internal/service/task/service.go`, dejar UNA query con `FROM tasks` sin renombrar a `issue_tasks` mientras la migración SÍ renombró la tabla.
   - **Esperado:** el test de integración del repositorio de tasks debe FALLAR con `relation "tasks" does not exist`.
   - Si pasa en verde → el path no está cubierto por tests; agregar cobertura.
3. **Restaurar:** descomentar el `ALTER ... RENAME CONSTRAINT` y revertir la query → ambos tests vuelven a verde.

## Cierre

- [ ] `go vet ./...` sin warnings
- [ ] `go build ./...` OK
- [ ] `go test ./...` verde (incluido el test de migración up/down y el schema_audit actualizado)
- [ ] `squawk` sobre `000151_...up.sql` y `000151_...down.sql` sin errores bloqueantes
- [ ] Grep final: `grep -rnE '(FROM|INTO|JOIN|UPDATE|DELETE FROM)\s+(requirements|proposals|designs|gherkin_scenarios|tasks|code_references|intake_payloads|verifications|verification_results|sabotage_records)\b'` en `internal/`, `cmd/`, `tests/` → 0 resultados (salvo `selfhosted_tasks`, que es otra tabla, y la CLAVE JSON `"sabotage_records"` en sdd_judge.go/registry.go, que NO es SQL)
- [ ] Verificación manual: aplicar migración en entorno local, correr el pipeline SDD de punta a punta (intake → req → issue → gherkin → proposal → design → task → verification) sin errores de relación
- [ ] Commit en rama `services` (Conventional Commits, español, SIN Co-Authored-By): `refactor(schema): rename dominio SDD/TDD e issue con prefijos (000151)`
- [ ] NO git push (repo local-only)
