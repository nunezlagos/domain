# Benchmarks — HU-27.4

Suite de benchmarks para hot-paths del runtime. Sirven como baseline
para detectar regresiones de performance.

## Correr

```bash
# Suite completa
go test -bench=. -benchmem -count=5 -run=^$ ./...

# Solo un paquete
go test -bench=. -benchmem -count=5 -run=^$ ./internal/api/cursor/

# Guardar baseline
go test -bench=. -benchmem -count=5 -run=^$ ./... > benchmark-results/main.txt

# Comparar
go test -bench=. -benchmem -count=5 -run=^$ ./... > benchmark-results/feature.txt
benchstat benchmark-results/main.txt benchmark-results/feature.txt
```

## Áreas cubiertas

| Paquete | Benchmark | Resultado típico (AMD Ryzen 7) |
|---|---|---|
| `internal/api/cursor` | CursorEncode | ~857 ns/op, 560 B, 5 allocs |
| | CursorDecode | ~1839 ns/op, 480 B, 9 allocs |
| | HashFilters (3 keys) | ~534 ns/op, 256 B, 5 allocs |
| `internal/api/etag` | Compute | ~402 ns/op, 256 B, 6 allocs |
| | LastModified | ~208 ns/op, 32 B, 1 alloc |
| `internal/dlock` | HashKey | ~114 ns/op, 64 B, 1 alloc |
| `internal/anonymizer` | FakerRUT | ~221 ns/op, 40 B, 3 allocs |
| | FakerEmail | ~153 ns/op, 40 B, 1 alloc |
| | RedactJSON (small) | ~2.6 µs, 1.6 KB, 27 allocs |
| | RedactJSON (nested) | ~5.1 µs, 3.5 KB, 55 allocs |

## Áreas pendientes

- Search global híbrido (HU-03.7) con dataset 100/1k/10k rows
- Agent run end-to-end (mock LLM)
- Flow run 5-step linear
- pgx query simple SELECT
- RLS overhead (con/sin policy)

Estas requieren testcontainers Postgres y son más costosas — correr manualmente
o en step weekly de CI, no en cada PR.

## Convenciones

- Benchmarks viven en `*_bench_test.go` (separados de `*_test.go` unit tests)
- `b.ResetTimer()` después del setup
- Sin I/O fuera del benchmark body
- `-run=^$` para skip unit tests al correr bench
- `-count=5` mínimo para reducir noise
