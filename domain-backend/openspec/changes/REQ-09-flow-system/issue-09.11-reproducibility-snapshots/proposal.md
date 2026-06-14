# Proposal: issue-09.11-reproducibility-snapshots

## Intención

Capturar snapshot determinístico al iniciar cada flow_run (flow_version, inputs, seed, frozen time, LLM params, skill versions) + endpoint replay que reproduce idéntico. Opt-in por flow.

## Scope

**Incluye:**
- Captura snapshot al boot del run
- Endpoint POST /runs/:id/replay con modo deterministic/with_overrides
- Frozen time helper en ExecContext
- Seeded random
- LLM response cache opcional para perfect replay
- Snapshot opt-in flag por flow

**No incluye:**
- Time-travel mid-execution (replay siempre desde el principio)
- Replay de signals/webhooks externos (se mockean en el snapshot opcionalmente)

## Enfoque técnico

1. Snapshot JSONB en flow_runs
2. `ExecContext.Now() time.Time` y `ExecContext.Rand() *rand.Rand` injectados
3. LLM call interceptor: si snapshot tiene cached_response → return cached
4. Replay endpoint crea nuevo run con snapshot importado

## Riesgos

- LLM provider model drift: documentar que replay NO garantiza identidad sin cache
- Snapshot bloat: comprimir + S3 si >1MB
- Steps con I/O externo no-mockable: marcar como `replay_warning`

## Testing

- Snapshot capturado al boot
- Replay deterministic: outputs match para steps puros
- Replay con override modifica solo lo overridden
- LLM cache hit en replay
- Opt-out: snapshot mínimo
