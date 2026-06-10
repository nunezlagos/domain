# Proposal: issue-27.4-feature-benchmarks

## Intención

Benchmark suite Go + benchstat comparativo + thresholds en CI + history tracking + SLO targets.

## Scope

- BenchmarkXxx files por feature crítico
- CI step: corre benchmarks + compara con baseline main
- benchstat threshold 10% regression
- Override mechanism PR body
- History CSV trend
- SLO targets check

## Riesgos

- CI flakiness: `-count=5` median; baseline update bajo control
- Slowness CI: benchmarks weekly (no PR), o subset rápido por PR

## Testing

- Benchmark fixture: regresión simulada → CI fail
- Override → CI pass con warning
- Trend CSV actualizado
- SLO target fail
