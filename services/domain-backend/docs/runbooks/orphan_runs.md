# Runbook: Orphan Runs Audit

## ¿Qué es?

El Orphan Runs Auditor es un system cron diario que detecta `agent_runs` huérfanos (sin `flow_run_id`) y expone métricas para alertar.

## ¿Cómo funciona?

1. Corre una vez al día por defecto (configurable via `DOMAIN_ORPHAN_AUDIT_SCHEDULE`, formato cron)
2. Consulta `agent_runs` con `flow_run_id IS NULL AND standalone IS NULL`
3. Excluye runs creados después del `last_ack_at` en `system_state`
4. Expone métrica `domain_agent_runs_orphan_total` para alertar

## ¿Qué NO cuenta?

- `standalone = true` — runs intencionalmente sin flow (tests, debugging)
- Runs con `flow_run_id IS NOT NULL` — están correctamente asociados
- Runs creados después del último `last_ack_at` — se evalúan en el próximo ciclo

## Métricas

- `domain_agent_runs_orphan_total` — runs huérfanos detectados
- `domain_orphan_audit_ticks_total{result}` — ticks del cron (ok|leader_skip|error)

## Cómo investigar

1. Verificar alerta `AgentRunsOrphanDetected` en Prometheus
2. Consultar `agent_runs WHERE flow_run_id IS NULL AND standalone IS NULL`
3. Decidir si son legítimos (bug de integración) o bypass intencional
4. Si es bypass: investigar source del INSERT sin flow_run_id
