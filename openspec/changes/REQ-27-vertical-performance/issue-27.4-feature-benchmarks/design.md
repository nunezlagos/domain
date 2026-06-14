# Design: issue-27.4-feature-benchmarks

## Setup

```go
// internal/search/search_bench_test.go
func BenchmarkSearch_Hybrid_10k(b *testing.B) {
  ctx, db := setupBenchDB(b, with10kObservations)
  b.ResetTimer()
  for i := 0; i < b.N; i++ {
    _, _ = service.Search(ctx, "postgres migration", nil)
  }
}
```

## CI workflow

```yaml
# .github/workflows/benchmarks.yml
on:
  pull_request:
  schedule:
    - cron: "0 4 * * 0"  # weekly main baseline update
jobs:
  bench:
    steps:
      - run: go test -bench=. -benchmem -count=5 -timeout=30m ./... > pr.txt
      - run: |
          curl -O https://github.com/.../benchmark-results/main.txt
          benchstat main.txt pr.txt > diff.txt
      - run: |
          if grep -q "FAIL" diff.txt; then exit 1; fi
          if check_regression diff.txt 10; then exit 1; fi
      - uses: peter-evans/create-or-update-comment@v3
        with: { body-file: diff.txt }
```

## SLO targets

```go
// internal/bench/targets.go
var slos = map[string]time.Duration{
  "BenchmarkSearch_Hybrid_10k":     500 * time.Millisecond,
  "BenchmarkAgentRun_E2E":          30 * time.Second,
  "BenchmarkFlowRun_5Step_Linear":  60 * time.Second,
  "BenchmarkPgxSelect_Simple":      5 * time.Millisecond,
  "BenchmarkMCPTool_CacheHit":      10 * time.Millisecond,
}
```

## TDD plan

1. Benchmark fixture corre
2. Regresión simulada (sleep added) → CI fail
3. Override en PR body → pass con warning
4. Trend CSV actualizado tras merge
5. SLO target violation → fail
