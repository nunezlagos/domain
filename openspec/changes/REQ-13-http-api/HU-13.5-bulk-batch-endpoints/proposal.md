# Proposal: HU-13.5-bulk-batch-endpoints

## Intención

Endpoints `/batch` para entidades de alta cardinalidad (observations, knowledge_docs, prompts) con Multi-Status response, modos all_or_nothing/best_effort, límite 5000 items.

## Scope

**Incluye:**
- POST /observations/batch, /knowledge_docs/batch, /prompts/batch
- DELETE /observations/batch
- Modes: all_or_nothing | best_effort (default)
- 207 Multi-Status response shape
- Idempotency compatible (HU-13.4)
- Size limit configurable (default 5000)
- Streaming response opcional para batches grandes

**No incluye:**
- Bulk update arbitrary fields (futuro)
- Async batches con callback (futuro)

## Enfoque técnico

1. Validation por item primero, antes de DB
2. all_or_nothing: tx + bulk insert con `pgx.CopyFrom`
3. best_effort: per-item tx; errors no abortan
4. Memory limit: stream parse JSON array

## Riesgos

- Long transactions: cap timeout 5min
- Memory: streaming parser
- N+1 events: batch event publication

## Testing

- 500 items happy path
- Item 250 falla en all_or_nothing → rollback
- best_effort: 5 fallan, 495 OK
- Size 5001 → 413
- Idempotency replay
- Bulk delete con permisos mixtos
