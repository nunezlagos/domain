# issue-27.4-feature-benchmarks

**Origen:** `REQ-27-vertical-performance`
**Prioridad tentativa:** baja
**Tipo:** tooling

## Historia de usuario

**Como** mantenedor
**Quiero** benchmark suite con baseline + comparativo en CI
**Para** detectar regresiones de performance antes de mergear

## Áreas a benchmarkear

| área | benchmark |
|------|-----------|
| Search global híbrido (issue-03.7) | 100/1k/10k observations dataset |
| Agent run end-to-end | 1 LLM call + 3 skills |
| Flow run | 5-step linear |
| pgx query simple SELECT | per-row latency |
| RLS overhead | con/sin policy |
| pgaudit overhead | con/sin |
| MCP tool call | con/sin cache hit |
| FTS+vector hybrid | per-query |

## Criterios de aceptación

### Escenario 1: Benchmark suite

```gherkin
Dado que existe `*_bench_test.go` con `BenchmarkXxx`
Cuando `go test -bench=. -benchmem -count=5 ./...`
Entonces se ejecutan todos los benchmarks
Y output guardado en `benchmark-results/<commit>.txt`
```

### Escenario 2: benchstat comparativo

```gherkin
Dado que existe baseline `benchmark-results/main.txt`
Cuando PR runs benchmarks
Entonces se compara con `benchstat main.txt pr.txt`
Y si >10% slower en cualquier benchmark sin override → CI fail
Y output como PR comment
```

### Escenario 3: Override explícito en PR

```gherkin
Dado que PR body incluye `perf-override: <benchmark>: justification`
Cuando CI procesa
Entonces override aceptado + audit
Y se mergea pero queda warning visible
```

### Escenario 4: Trend dashboard

```gherkin
Dado que existe `docs/benchmarks/history.csv` agregado tras cada merge
Cuando se actualiza
Entonces se ploteia trend (script Go o Grafana via Pushgateway)
Y se detectan regresiones graduales
```

### Escenario 5: SLO targets por feature

```gherkin
Dado que existen targets:
  | benchmark              | target p95 |
  | search hybrid 10k      | <500ms     |
  | agent run e2e          | <30s       |
  | flow run 5-step        | <60s       |
  | pgx select simple      | <5ms       |
  | MCP tool cache hit     | <10ms      |
Cuando benchmark supera target
Entonces CI fail con mensaje claro
```

## Análisis breve

- **Qué pide:** benchmark suite + benchstat + threshold + dashboard + targets
- **Esfuerzo:** M
- **Riesgos:** flaky benchmarks (CI noise); fix con `-count=5` y baseline update controlado
