# Proposal: HU-08.8-agent-parallel-fanout

## Intención

Permitir que un supervisor dispare N sub-agentes en paralelo con merge strategies built-in (first/all/vote/best-of-N/reduce-skill), timeout global y budget pool compartido.

## Scope

**Incluye:**
- Tool sintético `parallel_fanout(targets, instructions, context_keys, merge_strategy, timeout_seconds, total_budget_tokens, reduce_skill_slug?)`
- 5 merge strategies built-in implementadas en Go
- `errgroup` + ctx para concurrency + cancel cascade
- Budget pool: cap conjunto compartido entre sub-runs
- Partial results cuando timeout o errors

**No incluye:**
- Fan-out de skills (no agentes) — caso menor, ya cubierto por HU-09.2 step `parallel`
- Streaming partial outputs al supervisor (futuro)

## Enfoque técnico

1. Tool generator agrega `parallel_fanout` SI agent.subordinates no vacío
2. Engine usa `golang.org/x/sync/errgroup` con `WithContext`
3. Budget pool: atomic counter, cada sub-run consulta antes de cada LLM call
4. Merge strategies como funciones `func(outputs []Output) (Output, error)`
5. `cancel()` propagado a contexts hijos en first_completed

## Riesgos

- Memory: cap concurrent sub-runs (max 10 por fan-out)
- Cost runaway: budget pool enforcement + abort early
- Confusion vs HU-08.6 delegate: documentar cuándo usar cuál

## Testing

- 3 sub-agentes paralelos completan
- first_completed cancela los demás
- timeout cancela pendientes, devuelve partial
- error en 1 no rompe merge de los otros
- budget pool exceeded → cancel early
- custom reduce_skill invocado con outputs
- max 10 concurrent enforcement
