# Proposal: issue-13.4-idempotency-keys

## Intención

Middleware idempotency-key estilo Stripe: cliente envía `Idempotency-Key` en POST/PATCH/DELETE; el server cachea la response 24h y replay misma respuesta si key+body match.

## Scope

**Incluye:**
- Middleware HTTP que intercepta `Idempotency-Key` header
- Tabla `idempotency_records` con TTL 24h
- Lock para concurrent same-key (SELECT FOR UPDATE)
- Body hash SHA-256 para conflict detection
- Cron daily purge expired
- Header `Idempotent-Replayed: true` en cached response
- Aplicado solo a POST/PATCH/DELETE

**No incluye:**
- Auto-idempotency sin key explícita (futuro opcional)
- Cross-org idempotency sharing (scoped por org)

## Enfoque técnico

1. Middleware antes de handler: si key presente → lookup + lock + decide
2. Wrapper de ResponseWriter para capturar status/body
3. Persistir tras handler success (no si 5xx)
4. TTL cleaner cron

## Riesgos

- Storage bloat: cap 100k records/org + TTL
- Lock contention: timeout 30s
- Keys reusables: documented como caller responsibility

## Testing

- First request stores + responds
- Replay devuelve cached con header
- Body mismatch → 422
- Concurrent same key: 2do espera y cachea
- TTL expired → reprocess
- GET ignora header
