# Design: issue-08.12-orphan-runs-audit-cron

## Contexto

issue-08.10 ADR-2 define **enforcement híbrido reforzado**:
- service-layer rechaza orphan agent_runs (sin flow_run_id) en prod sin `WithStandalone(true)`
- ⚠️ pero alguien puede ejecutar `INSERT INTO agent_runs (flow_run_id=NULL, ...)` directo en BD (admin con acceso, test que use pool sin pasar por service, etc.)

Esta issue cierra ese gap con visibility — NO bloquea (sería trade-off de flexibilidad), pero AUDITA y alerta.

## ADR-1 — Cron diario, no continuo

Decisión: tick 1x al día (default 4am UTC).

Justificación:
- Bypass cases son raros (alguien tiene que tocar SQL directo)
- Latencia de detección 24h aceptable (no es seguridad crítica)
- 4am UTC = baja actividad global (Europe noche, US Pacific 8pm, Asia mediodía)
- Si necesita ser más frecuente, configurable via `DOMAIN_ORPHAN_AUDIT_SCHEDULE` (formato cron)

Alternativa rechazada: tick cada hora. Razón: overhead innecesario; el gap audita bypass humano, no rate-limited.

## ADR-2 — `metadata->>'standalone'` como signal

Definición del contract:

```go
// Cuando el service crea con WithStandalone(true):
agent_runs.metadata = {
  "standalone": true,
  "reason": "debug" | "script" | "test"
}
```

Query orphan:
```sql
SELECT id, organization_id FROM agent_runs
WHERE flow_run_id IS NULL
  AND (metadata->>'standalone' IS NULL OR metadata->>'standalone' != 'true')
  AND created_at > $1  -- last_ack_at
  AND created_at <= NOW();
```

Esto distingue:
- `standalone=true` legítimo (cliente intencional) → NO counta
- `flow_run_id NULL + sin metadata.standalone` (bypass) → SÍ counta

Risk: alguien que bypaseé escriba `metadata.standalone=true` manualmente para evadir auditoría. **Aceptado** — es lo mismo que documentar la práctica como standalone. Si va a esquivar, va a hacerlo bien.

## ADR-3 — Idempotencia via `last_ack_at`

Problema: si el cron corre 2x el mismo día (manual + scheduled), counta el mismo orphan 2x → inflated métrica.

Solución: persistir `last_ack_at` en tabla `system_state` (key='orphan_runs_audit'):

```sql
CREATE TABLE IF NOT EXISTS system_state (
  key VARCHAR(100) PRIMARY KEY,
  value JSONB NOT NULL DEFAULT '{}',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

Si esta tabla NO existe, agregarla en una migration 000074_create_system_state (mínima).

El tick:
1. Lee `last_ack_at` (default 24h atrás si no existe)
2. Query orphans con `created_at > last_ack_at`
3. Increment counter por cada
4. Update `last_ack_at = NOW()` atómicamente con el conteo

Trade-off: si el counter se pierde (Prometheus reinicia), recovery requiere replay. Aceptado — el alert eventualmente dispara igual.

## ADR-4 — Granularidad de cardinalidad

Métrica: `domain_agent_runs_orphan_total{org_id, reason}`.

- `org_id`: necesario para investigar qué org tiene el bypass
- `reason`: lista cerrada — `"bypass_service_layer"`, `"manual_insert"` (futuro si distinguimos)

Cardinalidad acotada (asumiendo <10k orgs + 2-3 reasons): OK por regla observability.md.

## ADR-5 — NO auto-cleanup

Decisión explícita: el cron **NO borra** orphans detectados. Sólo audita.

Justificación:
- Borrar agent_runs es destructivo (puede tener logs, costs vinculados)
- Si el orphan es legítimo (script de migración, script de admin), borrarlo rompería el script
- Auditoría + alert da contexto humano para decidir caso por caso

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|---|---|---|
| Counter explota cardinality con muchas orgs nuevas | Baja | Cap implícito <10k orgs |
| last_ack_at se corrompe → double-count | Baja | UPSERT atómico + valor por default |
| Cron NO corre por crash | Baja | k8s restart automático; alert si métrica no se actualiza en 48h |
| Falso negativo (orphan con standalone fake) | Media | Documentar contract; aceptar trade-off |

## Observabilidad

- `domain_agent_runs_orphan_total{org_id, reason}` — counter (definido en issue-08.10, incrementado acá)
- `domain_orphan_audit_ticks_total{result}` — counter del propio cron
- Log Info con count por tick
- Alert `AgentRunsOrphanDetected` en AlertManager

## Plan de implementación

1. Verificar/crear tabla `system_state` (migration mínima si no existe)
2. Implementar `OrphanAuditor` struct + Tick()
3. Tests unit
4. Tests integration (incluir sabotage INSERT bypass)
5. Wire-up en runServer
6. Alert en deploy/prometheus
