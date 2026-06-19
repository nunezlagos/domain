# issue-42.3-drop-legacy-infra

**Origen:** `REQ-42-schema-naming-taxonomy`
**Prioridad tentativa:** alta
**Tipo:** chore (limpieza destructiva de schema + remoción de código)

## Historia de usuario

**Como** arquitecto del sistema que aplica la taxonomía de naming (REQ-42)
**Quiero** dropear 8 tablas legacy/infra que NO encajan en ningún grupo de la taxonomía (`sessions`, `model_registry`, `entity_state_transitions`, `system_state`, `saga_compensation_log`, `runtime_configs`, `dead_letter_queue`, `idempotency_keys`) y remover/refactorizar TODO el código que las toca en el MISMO deploy
**Para** que el schema quede limpio, sin tablas huérfanas ni código muerto, y `domain-lint` + `go build` pasen verdes después del drop

## Alcance

Esta HU NO toca el cluster billing/costos (`plans`, `budgets`, `cost_logs`, `cost_alerts_sent`, `org_cost_alert_thresholds`) — eso es la HU 42.2. Acá SOLO las 8 tablas legacy/infra.

> **`sabotage_records` ya NO es parte de este drop:** por decisión del usuario se PRESERVA como capa TDD y se renombra a `tdd_sabotage_records` en la HU 42.5. En su lugar entra `saga_compensation_log` (mismo cluster saga/infra). El conteo neto sigue en 8 tablas.

Clasificación por riesgo (de la investigación):

| Tabla | safe_to_drop | Código vivo | FK que la complica |
|---|---|---|---|
| `entity_state_transitions` | **sí** | pkg `lifecycletrack` MUERTO (sin importers) | ninguna |
| `system_state` | no | cron `OrphanAuditor` (cursor `last_ack_at`) | ninguna |
| `saga_compensation_log` | **sí** | `flow/saga.go`, `heartbeat_watcher.go` | FK saliente → `flow_runs`; NADA la referencia (drop limpio) |
| `model_registry` | no | validación de modelos + pricing (`llm/registry`) | ninguna |
| `runtime_configs` | no | hot-reload backbone (registry + SIGHUP + cron + rutas) | ninguna |
| `dead_letter_queue` | no | `flow.DLQStore` inyectado en el flow runner | FK saliente → `flow_runs` |
| `idempotency_keys` | no | middleware HTTP en la cadena de TODA request mutante | FK saliente → `users` |
| `sessions` | no | **FEATURE COMPLETA** (pkg session, handler, MCP tools, timeline, search, lifecycle, stitcher) | **FK ENTRANTES**: `captured_prompts.session_id`, `verifications.session_id` |

> **OJO crítico de la investigación:** `runtime_configs`, `idempotency_keys` y `dead_letter_queue` tienen CÓDIGO VIVO en la cadena de boot/HTTP. El drop NO compila ni bootea si la migration corre sin que el código que las usa se remueva/refactorice en el MISMO deploy. Ver `tasks.md` (secciones de remoción) y `design.md` (orden de deploy).

## Criterios de aceptación

```gherkin
Feature: Drop de 8 tablas legacy/infra (REQ-42 taxonomía)

  Background:
    Given la DB está en estado post-Fase-C (single-org, sin organization_id)
    And las 8 tablas tienen 0 filas (verificado en introspección)

  Scenario: La migration 000149 dropea las 8 tablas de forma idempotente
    When aplico la migration 000149 up
    Then no existen las tablas sessions, model_registry, entity_state_transitions, system_state, saga_compensation_log, runtime_configs, dead_letter_queue, idempotency_keys
    And re-aplicar la migration es no-op (DROP TABLE IF EXISTS)
    And la migration corre dentro de una sola transacción BEGIN/COMMIT

  Scenario: Las FK constraints y columnas entrantes a sessions se limpian antes del drop
    Given captured_prompts.session_id y verifications.session_id apuntan a sessions
    When aplico la migration 000149 up
    Then se dropean explícitamente las constraints captured_prompts_session_id_fkey y verifications_session_id_fkey
    And la columna session_id ya no existe en captured_prompts
    And la columna session_id ya no existe en verifications
    And el DROP TABLE sessions no falla por dependencias

  Scenario: Objetos derivados de entity_state_transitions caen con la tabla
    Given existe la vista v_stuck_entities y el trigger append-only
    When aplico la migration 000149 up con CASCADE
    Then la vista v_stuck_entities ya no existe
    And la función entity_state_transitions_immutable() ya no existe
    And la sequence entity_state_transitions_id_seq ya no existe

  Scenario: El backend compila y bootea sin las tablas
    Given el código vivo de runtime_configs, idempotency_keys, dead_letter_queue, system_state, model_registry, sessions, saga_compensation_log fue removido/refactorizado
    When corro go build ./... y go vet ./...
    Then compila sin errores y sin warnings
    And el binario bootea sin construir el Registry de runtimeconfig ni el idempotency middleware ni el DLQStore

  Scenario: Crear un agente NO depende de model_registry
    Given se removió validateModel/ModelExists
    When creo un agente con un modelo válido del cliente Anthropic
    Then el agente se crea sin consultar model_registry
    And no se devuelve ErrModelUnknown por falta de tabla

  Scenario: El down restaura el schema post-Fase-C (no los datos)
    When aplico la migration 000149 down
    Then las 8 tablas existen de nuevo con su shape post-Fase-C (sin organization_id, con status)
    And captured_prompts.session_id y verifications.session_id vuelven a existir como FK nullable ON DELETE SET NULL
    But los datos NO se restauran (las tablas estaban vacías; restore real vía pgBackRest)

  Scenario: domain-lint pasa después del cambio
    When corro el linter de migrations (squawk + dbconvlint)
    Then no hay referencias a tablas inexistentes en commonNonPluralAllowed ni en ejemplos
    And la migration 000149 pasa squawk (DROP en tabla vacía, sin lock largo)
```

## Análisis breve

- **Qué pide realmente:** drop atómico de 8 tablas + extirpar el código que las usa para que el sistema siga compilando y booteando. NO es un drop puro: 6 de las 8 tienen acoplamiento vivo.
- **Por qué `sessions` es el de mayor riesgo:** es una feature completa con stack propio (pkg session, handler, MCP tools) y además es LEÍDA por servicios que se conservan (`timeline`, `search`, `lifecycle`, `stitcher`). Hay que romper esas dependencias sin romper esos paquetes KEEP. La investigación recomienda DISPUTAR el drop o tratarlo como épica aparte — lo dejo documentado en "Disputas".
- **Por qué `runtime_configs` / `idempotency_keys` / `dead_letter_queue` son trampa:** la migration sola dropea la tabla, pero el binario YA NO BOOTEA (runtimeconfig.Refresh al arranque, idempMW.Wrap en la cadena HTTP, DLQStore inyectado en el runner). Drop de DB y remoción de código DEBEN ir en el mismo deploy.
- **Único drop realmente safe:** `entity_state_transitions` (su único consumidor, `lifecycletrack`, es código muerto sin importers).
- **Esfuerzo tentativo:** L (drop trivial; el peso está en la cirugía de código de 6 tablas).

## Decisiones (resueltas con el usuario)

1. **`sabotage_records`**: RESUELTO — se PRESERVA como `tdd_sabotage_records` (capa TDD, mutation testing). **Sale de este drop**; el rename va en la HU 42.5. Su código (`task/service.go` CreateSabotage/ListSabotages, `sdd_judge.go`, `registry.go`) NO se remueve aquí: solo se actualizan los literales SQL al nuevo nombre en la HU 42.5.
2. **`saga_compensation_log`**: RESUELTO — se DROPEA (mismo cluster saga/infra). Entra en este lote en lugar de `sabotage_records`. FK saliente `run_id → flow_runs`; nada la referencia → drop limpio sin CASCADE.
3. **`sessions`**: drop implica desmontar una feature entera + romper dependencias en 4 paquetes KEEP. ¿Se confirma el drop o se trata como épica separada? (sigue abierta)
4. **`runtime_configs`**: ¿se elimina el feature de hot-reload completo, o se reimplementa sobre `flow_config`/env? Esta HU asume ELIMINAR el feature (el más simple y coherente con single-org).

## Verificación previa

- [ ] Confirmar que las 8 tablas tienen 0 filas en el VPS (introspección: confirmado, todas en 0)
- [ ] Confirmar que `captured_prompts.session_id` y `verifications.session_id` siguen siendo las únicas FK entrantes a `sessions`
- [ ] Confirmar que el pkg `lifecycletrack` no tiene NINGÚN importer (`grep -rn "lifecycletrack" --include=*.go`)
- [ ] Confirmar que `system_state` solo lo usa el `OrphanAuditor` (cursor) y `dbschema.go` (clasificación)
- [ ] Confirmar que tras remover el código, `go build ./...` y `go vet ./...` pasan
- [ ] Confirmar que el binario bootea sin `runtimeconfig.Registry`, sin `idempMW`, sin `DLQStore`
- [ ] Confirmar que `model_registry` ya no gobierna la creación de agentes (sin `ErrModelUnknown`)
- [ ] Confirmar que NADA referencia `saga_compensation_log` (0 FK entrantes → drop limpio sin CASCADE)
- [ ] Confirmar decisiones: `sabotage_records` PRESERVADA (rename 42.5), `saga_compensation_log` DROP; pendientes `sessions` / `runtime_configs` ANTES de mergear

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
