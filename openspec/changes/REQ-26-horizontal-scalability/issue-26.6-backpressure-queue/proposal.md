# Proposal: issue-26.6-backpressure-queue

## Intención

Caps globales y per-org en colas + 429 shed-load + métricas + drop policy declarada para colas non-critical.

## Scope

- Migration agrega config per-queue
- Middleware/check antes de INSERT en queues
- Métricas gauges depth
- Per-org quota integrada con issue-21.3 plans
- Drop policy field

## Riesgos

- Calibration: thresholds defaults conservadores + observability primero
- Drop policy en wrong queue: explicit `drop_policy: none|fifo` config

## Testing

- Cap global → 429
- Cap per-org → 429
- Métricas observables
- Drop fifo en queue marcada
