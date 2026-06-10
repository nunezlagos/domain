# Testing Conventions — Domain

Tests son first-class. CI bloquea merge si rojos (issue-19.1).

## Estructura

Por feature, no por tipo:

```
internal/service/agent/
  service.go
  service_test.go         ← unit tests con mocks
internal/store/pg/agent/
  store.go
  store_test.go           ← integration con Postgres real (testcontainers)
internal/http/handlers/
  agent.go
  agent_test.go           ← HTTP tests con httptest
```

## Tipos de tests

| tipo | herramientas | corren en |
|------|--------------|-----------|
| Unit | `testing` + `testify/require` | toda build (`go test ./...`) |
| Integration DB | `testcontainers-go/postgres` | tag `integration`, CI separate step |
| Integration HTTP | `httptest.NewServer` + cliente real | tag `integration` |
| E2E | docker-compose stack + curl/client | manual + CI weekly |
| Benchmark | `testing.B` | manual + CI weekly |
| Property-based | `gopter` para invariantes | opcional |

## Build tags

```go
//go:build integration
// +build integration
```

CI corre:
- PR: `go test ./...` + `go test -tags=integration ./...`
- Main: igual + benchmarks weekly

## Naming

```go
func TestUserService_CreateUser_HappyPath(t *testing.T) {}
func TestUserService_CreateUser_DuplicateEmail_Returns409(t *testing.T) {}
func TestUserService_CreateUser_InvalidRUT_ReturnsValidation(t *testing.T) {}
```

Formato: `Test<Subject>_<Method>_<Scenario>_<ExpectedOutcome>`.

## Assertions

- `require.NoError(t, err)` (no `assert.Nil(t, err)`)
- `require.Equal(t, expected, actual)` — `expected` primero
- `require.Len(t, slice, N)` mejor que `Equal` para length
- `assert` para non-fatal, `require` para must-fail

## Subtests con t.Run

Tests data-driven:

```go
cases := []struct {
  name    string
  input   string
  want    string
  wantErr bool
}{
  {"valid rut", "12345678-5", "12345678-5", false},
  {"invalid dv", "12345678-9", "", true},
  {"with dots", "12.345.678-5", "12345678-5", false},
}
for _, tc := range cases {
  t.Run(tc.name, func(t *testing.T) {
    got, err := rut.Normalize(tc.input)
    if tc.wantErr { require.Error(t, err); return }
    require.NoError(t, err)
    require.Equal(t, tc.want, got)
  })
}
```

## Parallel

- `t.Parallel()` en tests independientes
- NO en integration tests que comparten DB instance salvo schemas separados

## testcontainers

```go
func setupPostgres(t *testing.T) *pgxpool.Pool {
  t.Helper()
  ctx := context.Background()
  pgC, err := postgres.RunContainer(ctx,
    testcontainers.WithImage("pgvector/pgvector:pg16"),
    postgres.WithDatabase("domain"),
    postgres.WithUsername("test"),
    postgres.WithPassword("test"),
  )
  require.NoError(t, err)
  t.Cleanup(func() { pgC.Terminate(ctx) })
  // run migrations
  // return pool
}
```

Cada test integration crea su container, o usa singleton con cleanup per-test (DELETE rows).

## Fixtures

- `testdata/` directorio standard Go
- Fixtures JSON/YAML para inputs complejos
- Helper `loadFixture(t, "users/bob.json")`
- NUNCA datos PII reales (usar fakers)

## Mocking

- Mocks generados con `mockery` para interfaces de `domain/`
- Mocks viven en `internal/<feature>/mocks/` (no en mismo file)
- NO mockear `time.Now()` — usar `Clock` interface inyectado
- NO mockear `random` — usar seeded `*rand.Rand` inyectado

## Sabotaje pattern

Cada HU incluye al menos 1 test "sabotaje":
- Romper invariante intencional → verificar test catch
- Confirma que el test no es "always green"

Ejemplo:
```go
func TestSabotage_ScopeWithoutOrgID_Detected(t *testing.T) {
  // intenta query sin SET LOCAL → debe devolver 0 rows (RLS)
}
```

## Coverage

- Target: 70% global, 80% en `service/` y `domain/`
- issue-19.1 publica coverage en PR
- Coverage NO baja >2% sin justificación

## Performance

- Benchmarks en `*_test.go` con `BenchmarkXxx`
- Comparativo: `benchstat` entre `main` y PR
- Tests críticos: <100ms p95
- Integration tests: total CI <10min

## Determinismo

- Tests NO dependen de:
  - `time.Now()` real → inyectar Clock
  - `math/rand` global → inyectar `*rand.Rand`
  - network externa → mocks o testcontainers
  - filesystem fuera de `t.TempDir()`
- Tests NO mutan estado global compartido

## Anti-patterns prohibidos

- ❌ Sleep en lugar de retry/wait
- ❌ `if !condition { t.Fatal() }` cuando `require.True` es mejor
- ❌ Skip si "no se puede testear" — siempre se puede, refactor primero
- ❌ Test que toca prod DB
- ❌ Mocks de `time.Now()` con monkey-patching → inyectar interface
- ❌ Sin `t.Cleanup()` para resources
- ❌ Tests interdependientes (orden importa)
- ❌ Tests que solo testean el mock (no la lógica)
- ❌ Magic numbers sin constantes
