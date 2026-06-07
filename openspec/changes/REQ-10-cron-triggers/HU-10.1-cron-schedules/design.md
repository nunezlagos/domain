# Design: HU-10.1-cron-schedules

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativas |
|----------|---------------|--------------|
| Cron parser | `robfig/cron/v3` | `cronexpr` (robfig es estándar de facto, soporta timezones nativamente) |
| Scheduler loop | Ticker cada 60s con query SQL | Postgres pg_cron extensión (dependency externa; prefiero control en app) |
| Prevención de doble ejecución | `SELECT ... FOR UPDATE SKIP LOCKED` | Lock a nivel de app (DB lock es más robusto para múltiples réplicas) |
| Overlap handling | Skip si previous sigue running | Queue with backpressure (skip es simple y suficiente para MVP) |

## Alternativas descartadas

- **pg_cron**: Extensión de Postgres que corre crons internamente. No tenemos control sobre la lógica de ejecución (disparar flows/agentes requiere llamadas a la app). Lo descartamos porque la ejecución necesita nuestro runtime.
- **Worker externo (Celery-like)**: Innecesario para MVP. Un scheduler en una goroutine es suficiente. Si escalamos a múltiples réplicas, el FOR UPDATE SKIP LOCKED garantiza que solo una réplica ejecute cada cron.

## Diagrama

```
┌─────────────────┐     cada 60s
│ Scheduler       │◄══════════════
│ (goroutine)     │               │
└────────┬────────┘               │
         │                        │
         ▼ timer tick             │
┌──────────────────┐              │
│ SELECT * FROM    │              │
│ crons WHERE      │              │
│ enabled=true AND │              │
│ next_run <= NOW()│              │
│ FOR UPDATE SKIP  │              │
│ LOCKED           │              │
└────────┬─────────┘              │
         │                        │
         ▼ por cada cron          │
┌──────────────────┐              │
│ ¿Ejecución previa │── sí ──► skip (log)
│ sigue running?   │              │
└────────┬─────────┘              │
         │ no                     │
         ▼                        │
┌──────────────────┐
│ Iniciar ejecución │
│ flow o agente     │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Actualizar cron:  │
│ last_run = now    │
│ next_run = calc() │
└────────┬─────────┘
         │
         ▼
┌──────────────────┐
│ Insertar registro │
│ cron_executions   │
└──────────────────┘
```

Modelo `crons`:
```sql
CREATE TABLE crons (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    cron_expression VARCHAR(100) NOT NULL,
    flow_slug VARCHAR(255),
    agent_slug VARCHAR(255),
    project_id UUID NOT NULL REFERENCES projects(id),
    timezone VARCHAR(100) NOT NULL DEFAULT 'UTC',
    enabled BOOLEAN NOT NULL DEFAULT true,
    params JSONB DEFAULT '{}',
    last_run TIMESTAMPTZ,
    next_run TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT cron_target_check CHECK (
        (flow_slug IS NOT NULL AND agent_slug IS NULL) OR
        (flow_slug IS NULL AND agent_slug IS NOT NULL)
    )
);

CREATE INDEX idx_crons_next_run ON crons(enabled, next_run) WHERE enabled = true;

CREATE TABLE cron_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cron_id UUID NOT NULL REFERENCES crons(id) ON DELETE CASCADE,
    scheduled_at TIMESTAMPTZ NOT NULL,
    executed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    status VARCHAR(50) NOT NULL DEFAULT 'running',
    flow_run_id UUID REFERENCES flow_runs(id),
    agent_run_id UUID REFERENCES agent_runs(id),
    error TEXT,
    duration_ms INT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

## TDD plan

1. **Red:** Test `TestParseCronExpression_Valid` — expresión válida no retorna error
2. **Green:** Integrar robfig/cron parser
3. **Red:** Test `TestParseCronExpression_Invalid` — expresión inválida retorna error
4. **Green:** Validar expresión
5. **Red:** Test `TestCalculateNextRun_UTC` — next_run correcto en UTC
6. **Green:** Calcular next_run con parser
7. **Red:** Test `TestCalculateNextRun_WithTimezone` — next_run en AR-TZ
8. **Green:** Usar time.LoadLocation para convertir
9. **Red:** Test `TestScheduler_ExecutesDueCron` — scheduler ejecuta cron debido
10. **Green:** Implementar scheduler worker
11. **Red:** Test `TestScheduler_SkipDisabled` — cron disabled no ejecuta
12. **Green:** Filtrar enabled=true en query
13. **Red:** Test `TestScheduler_PreventsDoubleExecution` — FOR UPDATE evita duplicado
14. **Green:** Usar FOR UPDATE SKIP LOCKED
15. **Red:** Test `TestCronHistory` — historial se registra
16. **Green:** Insertar cron_executions en cada ejecución
17. **Sabotaje:** Sacar FOR UPDATE → test de doble ejecución falla

## Riesgos y mitigación

| Riesgo | Probabilidad | Impacto | Mitigación |
|--------|-------------|---------|------------|
| Doble ejecución | Media | Alto | FOR UPDATE SKIP LOCKED + idempotencia en flow/agent runner |
| Scheduler caído | Baja | Alto | Segunda goroutine de health check; si no ejecuta en 2 min, alertar |
| DST gap o ambigüedad | Baja | Medio | robfig/cron maneja DST; probar con timezone Chile (DST complejo) |
| Cron con intervalo < 1min | — | — | No soportado (mínimo 1 minuto). Documentar limitación. |
