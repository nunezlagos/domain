# Proposal: issue-25.4-schema-drift

## Intención

Cron diario que compara schema real de prod vs el schema esperado aplicando migrations a DB clean, reporta drift, paginar si encuentra.

## Scope

**Incluye:**
- Cron Kubernetes Job 1x/day
- Spawn ephemeral Postgres + apply migrations (expected schema)
- pg_dump --schema-only ambas
- Diff normalizado (apgdiff o pg-schema-diff)
- Notif + S3 spill diff completo
- Endpoint admin último resultado

**No incluye:**
- Auto-fix drift (manual)
- Continuous monitoring (24h cron suficiente)

## Enfoque técnico

1. Cron usa kind/k3s o pg testcontainer
2. pg_schema_diff o `apgdiff` para normalize
3. Threshold: any non-comment diff → notif

## Riesgos

- Falsos positivos por orden de pg_dump: usar `--schema-only --no-owner --no-privileges` + sort
- Ephemeral DB cost: cron es 1x/day, costo bajo

## Testing

- Drift simulado (ALTER manual prod) → detecta
- No drift → métrica ok
- Migration parcial dirty → detecta
- Endpoint serves último resultado
