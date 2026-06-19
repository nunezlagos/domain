# issue-42.5-rename-sdd-tdd

**Origen:** `REQ-42-schema-naming-taxonomy`
**Prioridad tentativa:** alta
**Tipo:** refactor (schema naming — breaking interno, API pública sin cambios)

## Historia de usuario

**Como** arquitecto del schema de `domain`
**Quiero** renombrar las tablas del pipeline SDD/TDD y de la capa issue para que TODAS lleven el prefijo de su funcionalidad (`sdd_`, `tdd_`, `issue_`)
**Para** poder agruparlas visualmente en el admin `/database`, hacer el dominio TDD VISIBLE (hoy escondido en `verifications`/`verification_results`) y alinear de paso los nombres legacy `*_hu_id_*` que sobrevivieron al rename de columnas de la migración 000073

## Alcance exacto (11 tablas)

| Grupo | Tabla actual | Tabla nueva |
|---|---|---|
| `sdd_` | `requirements` | `sdd_requirements` |
| `sdd_` | `proposals` | `sdd_proposals` |
| `sdd_` | `designs` | `sdd_designs` |
| `issue_` | `gherkin_scenarios` | `issue_gherkin_scenarios` |
| `issue_` | `tasks` | `issue_tasks` |
| `issue_` | `code_references` | `issue_code_references` |
| `issue_` | `intake_payloads` | `issue_intake_payloads` |
| `tdd_` | `verifications` | `tdd_verifications` |
| `tdd_` | `verification_results` | `tdd_verification_results` |
| `tdd_` | `sabotage_records` | `tdd_sabotage_records` |

> **Por qué `issue_` y no `sdd_` para gherkin/tasks/code_references/intake:** estas tablas cuelgan por FK del `issue` (no del REQ ni del proposal). `gherkin_scenarios` es el contrato de aceptación BDD del issue; `tasks` y `code_references` son la ejecución del issue; `intake_payloads` se *commitea* hacia issue/req. Pertenencia por FK manda sobre la fase del pipeline.

> **Por qué `tdd_` para verifications/verification_results:** la capa TDD del producto YA existe pero está escondida bajo nombres genéricos. `verifications` = checkpoints (build/test/lint/smoke/typecheck/migration) que el LLM dispara; `verification_results` = evidencia pass/fail por tarea. Renombrarlas las hace agrupables y descubribles.

> **Por qué `tdd_` para sabotage_records:** es mutation/sabotage testing (FK viva a `tasks`/`issue_tasks`: action/expected_failure/actual_result/restored). Por decisión del usuario se PRESERVA como parte del flujo SDD/capa TDD (junto a `tdd_verifications`/`tdd_verification_results`). Se renombra a `tdd_sabotage_records` (sin sequence: `id` no-serial). Su FK saliente `task_id → issue_tasks` queda consistente por OID en este mismo lote.
>
> **OJO — no confundir con el JSON key:** `sdd_judge.go` (l.48/63/65) y `registry.go` (l.115) usan `"sabotage_records"` como CLAVE de un payload JSON del judge, NO como nombre de tabla SQL. Esas referencias NO se tocan. Solo se renombran los literales SQL reales (`task/service.go` l.327/342).

## Criterios de aceptación

```gherkin
Feature: Rename taxonómico del dominio SDD/TDD/issue (migración 000151)

  Background:
    Given la migración 000146 ya está aplicada (última = 146)
    And la base es single-org y las tablas afectadas tienen 0 filas

  Scenario: Migración up renombra las 11 tablas en una sola transacción
    When aplico la migración 000151 up
    Then existen las tablas sdd_requirements, sdd_proposals, sdd_designs
    And existen issue_gherkin_scenarios, issue_tasks, issue_code_references, issue_intake_payloads
    And existen tdd_verifications, tdd_verification_results, tdd_sabotage_records
    And NO existen las tablas requirements, proposals, designs, gherkin_scenarios, tasks, code_references, intake_payloads, verifications, verification_results, sabotage_records

  Scenario: Las FKs entre tablas renombradas sobreviven por OID
    Given proposals y designs se renombran en el mismo lote
    When aplico la migración 000151 up
    Then la FK sdd_designs.proposal_id sigue apuntando a sdd_proposals
    And la FK tdd_verification_results.task_id sigue apuntando a issue_tasks
    And la FK tdd_sabotage_records.task_id sigue apuntando a issue_tasks
    And la FK issue_gherkin_scenarios.issue_id sigue apuntando a issues

  Scenario: Los nombres de índices y constraints quedan re-prefijados y sin legacy hu_id
    When aplico la migración 000151 up
    Then el constraint de unicidad de sdd_proposals se llama sdd_proposals_issue_id_version_key
    And NO existe ningún índice o constraint con substring "_hu_id_" en estas tablas
    And NO existe ningún índice tdd_verifications con substring "_org_" en el nombre

  Scenario: El trigger genérico sobrevive y el legacy se recrea
    When aplico la migración 000151 up
    Then issue_intake_payloads conserva el trigger trg_set_updated_at (genérico, intacto)
    And issue_intake_payloads tiene el trigger set_updated_at_issue_intake_payloads
    And NO existe el trigger set_updated_at_intake_payloads

  Scenario: Down revierte exactamente al estado previo
    Given apliqué la migración 000151 up
    When aplico la migración 000151 down
    Then las 10 tablas vuelven a sus nombres originales
    And los constraints vuelven a sus nombres legacy (incluido _hu_id_)
    And el trigger set_updated_at_intake_payloads vuelve a existir

  Scenario: El backend desplegado en el mismo deploy usa los nombres nuevos
    Given el código de los services fue actualizado a los nombres nuevos
    When el backend ejecuta cualquier query del pipeline SDD/TDD
    Then no hay errores "relation ... does not exist"
    And el e2e schema_audit_test refleja los nombres nuevos en expectedTables

  Scenario: squawk aprueba la migración
    When corro squawk sobre 000151 up y down
    Then no reporta errores bloqueantes (los RENAME son metadata-only, sin lock de tabla largo)
```

## Tablas SDD/TDD faltantes (resumen — detalle en design.md)

La taxonomía concluye que **NO se justifica crear ninguna tabla nueva** en este lote:

- **NO** `tdd_test_cases` ni `tdd_coverage` → el server es stateless respecto a la ejecución (el LLM corre tests con Bash/Read y reporta vía `items` JSONB de `tdd_verifications`).
- **NO** `sdd_acceptance_criteria` → `issue_gherkin_scenarios` ya cumple ese rol.
- **NO** puente issue↔ticket → `project_tickets.linked_issue_id` ya lo resuelve.

`sabotage_records` (mutation/sabotage testing, FK viva a `tasks`/`issue_tasks`) se PRESERVA por decisión del usuario y SÍ entra en este lote: se renombra a `tdd_sabotage_records` (capa TDD). Como esta migración renombra `tasks` → `issue_tasks` en la misma tx, su FK saliente `task_id` queda consistente por OID.

## Análisis breve

- **Qué pide realmente:** un rename atómico (estilo 000146 + 000073) de 10 tablas + sus índices/constraints + un trigger legacy, más la actualización coordinada de TODO el SQL embebido del backend en el MISMO deploy.
- **Módulos a tocar (SQL embebido):** `service/spec`, `service/requirement`, `service/issue`, `service/task` (incluye los literales de `sabotage_records` → `tdd_sabotage_records`), `service/traceability`, `service/intake`, `mcp/server/{proposals_tools,verifications_tools,health_tools}`, `api/handler/{spec,rest_new,api,requirement}`, `runner/intake/worker`, `cmd/domain/seed_demo.go`, `tests/e2e/schema_audit_test.go`. Lista completa en tasks.md.
- **Riesgos / dependencias:** falsos positivos al renombrar `tasks` (palabra común en Go; filtrar a literales SQL). Coordinación de FKs en la misma tx. Deploy atómico migración + código.
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Confirmar que 000146 es la última migración aplicada y 000151 es el número asignado a esta HU (147-150 reservados a HUs hermanas 42.1-42.4)
- [ ] Confirmar contra la introspección los nombres REALES de índices/constraints (los `*_hu_id_*` legacy existen; NO asumir prefijo de tabla)
- [ ] Confirmar que `verifications` ya NO tiene columna `organization_id` ni policy RLS (dropeadas en 000132)
- [ ] Confirmar que `intake_payloads` tiene los DOS triggers (`trg_set_updated_at` genérico + `set_updated_at_intake_payloads` legacy)
- [ ] Confirmar que ninguna de las 10 tablas tiene RLS policy vigente (sólo `audit_log`/`otp_codes` la tienen)
- [ ] Grep de `tasks` como literal SQL (FROM/INTO/UPDATE/JOIN/DELETE FROM) separado de identificadores Go
- [ ] Confirmar que `selfhosted_tasks` NO se toca (es otra tabla)

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
