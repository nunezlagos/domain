# Design: HU-15.3-usage-alerts

## Decisión arquitectónica

**Evaluación híbrida:** Evaluación post-ejecución para cost_per_run (inmediata, contextual) y evaluación periódica (cada 60s) para métricas acumulativas (cost_per_day, tokens_per_minute).

**Notifier interface:** Permite agregar nuevos canales sin modificar el evaluador. Cada notifier implementa `Send(ctx, alert, value)`.

**Alert log table:**
```sql
CREATE TABLE alert_log (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_id   UUID NOT NULL REFERENCES alerts(id),
    metric     VARCHAR(50) NOT NULL,
    threshold  DECIMAL(12,6) NOT NULL,
    value      DECIMAL(12,6) NOT NULL,
    channel    VARCHAR(20) NOT NULL,
    sent_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status     VARCHAR(20) NOT NULL, -- sent, failed
    error_msg  TEXT
);
```

## Diagrama

```
Trigger: Post-execution (agent/flow run)
  │
  ▼
Evaluator.EvaluateRun(runID)
  ├── get run cost from token_usage
  ├── check alerts with metric = cost_per_run
  ├── if should fire → notifier.Send()
  └── log to alert_log

Trigger: Periodic ticker (every 60s)
  │
  ▼
Evaluator.EvaluatePeriodic()
  ├── get cost_per_day from token_usage
  ├── get tokens_per_minute
  ├── check alerts for each metric
  ├── if should fire → notifier.Send()
  └── log to alert_log
```

## TDD plan

1. **Red:** Test `TestAlert_Evaluate_CostPerRun` excede threshold
2. **Green:** Implementar evaluador básico
3. **Refactor:** Extraer Notifier interface, metric getters
4. **Iterar:** Debounce, email/webhook, alert log
5. **Sabotaje:** Evaluador ignorando cooldown → test detecta

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| Email SMTP configurado incorrectamente | Falla silenciosa + log, no detiene el evaluador |
| Webhook flooding | Debounce + cola de notificaciones con rate limit |
| Alerta se dispara en cada evaluación si condición sigue true | Debounce: solo disparar si last_fired_at + cooldown < now |
