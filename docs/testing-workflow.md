# Domain — Flujo TDD y Validación por HU

> Cada HU sigue el ciclo `test → impl mínima → refactor → sabotaje`. Sin tests no se mergea. Sin sabotaje no se considera implementada.

## Reglas (resumen de `.claude/rules/sdd.md` + `testing.md`)

1. **Test primero** — el test falla `red` antes de existir la implementación
2. **Impl mínima** — solo lo necesario para pasar el test
3. **Refactor** — limpieza + conventions, tests siguen verdes
4. **Sabotaje** — romper invariante intencional, confirmar que test atrapa, restaurar
5. **CI** — bloquea merge si cualquier test falla, lint falla, o coverage cae >2%
6. **Coverage target**: 70% global, 80% `service/` + `domain/`

## Niveles de test

| nivel | herramientas | cuándo |
|-------|--------------|--------|
| Unit | `testing` + `testify/require` | toda función pública de `service/` y `domain/` |
| Integration DB | `testcontainers-go/postgres` con build tag `integration` | repos `store/pg/`, migrations |
| Integration HTTP | `httptest` | handlers `http/handlers/` y `api/handler/` |
| E2E | docker-compose + curl/SDK | weekly CI + pre-release manual |
| Sabotaje | en cualquier nivel | al menos 1 por HU |

## Build tags

```go
//go:build integration
// +build integration
```

Comandos:
```bash
make test                  # unit only (rápido, sin DB)
make test-integration      # con testcontainers
make test-all              # ambos
```

## Workflow per HU

### Step 1: Leer la HU

```bash
cat openspec/changes/REQ-XX/HU-XX.Y/hu.md
cat openspec/changes/REQ-XX/HU-XX.Y/design.md
cat openspec/changes/REQ-XX/HU-XX.Y/tasks.md
```

Identificar:
- **Persona** que sirve (header)
- **Escenarios Gherkin** (uno por test al menos)
- **Análisis breve** (módulos sospechados, riesgos)

### Step 2: Crear test files antes que impl

```
internal/<feature>/<feature>_test.go        # unit, mocks
internal/store/pg/<feature>/store_test.go    # integration con DB
```

### Step 3: Escribir tests que reflejen Gherkin

Mapeo Gherkin → Go test:

```gherkin
Escenario 1: foo bar
  Dado X
  Cuando Y
  Entonces Z
```

→

```go
func TestFoo_Bar_HappyPath(t *testing.T) {
  // arrange: X
  // act: Y
  // assert: Z
}
```

### Step 4: Confirmar red

```bash
go test ./internal/<feature>/...
# debe fallar — pero por la razón correcta (función no existe / mock setup), no syntax error
```

### Step 5: Implementación mínima

Suficiente para que el test pase. No sobre-engineer.

### Step 6: Confirmar green

```bash
go test ./internal/<feature>/...
# todos pasan
```

### Step 7: Refactor

Aplicar:
- `.claude/rules/go.md` — pgx v5, slog, errores con `fmt.Errorf("...:%w", err)`
- `.claude/rules/clean-architecture.md` — interfaces en consumer
- `.claude/rules/db.md` — naming/types/FKs (si toca SQL)
- `.claude/rules/security.md` — no PII en logs
- `.claude/rules/api.md` — response shape (si toca HTTP)

### Step 8: Sabotaje test

Cada HU declara al menos UN test sabotaje (ver tabla en `hu.md`).

```go
func TestSabotage_X_Detected(t *testing.T) {
  // arrange: condición que VIOLA el invariante
  // assert: que el sistema/test atrapa la regresión
}
```

Ejemplos:
- HU-25.5 RLS: tx sin `SET LOCAL` → 0 rows
- HU-17.1 metrics: label `_id="<uuid>"` en /metrics → linter fail
- HU-17.3 logging: `slog.String("password", x)` → PII linter fail
- HU-02.7 OTP: 6 intentos incorrectos → 429 `too_many_attempts`

### Step 9: Verificar coverage

```bash
go test -coverprofile=coverage.out ./internal/<feature>/...
go tool cover -func=coverage.out | grep -v 100.0%
# revisar funciones bajo coverage target
```

### Step 10: Commit

Conventional Commits (ver `.claude/rules/git.md`):

```
feat(req-XX): implementa HU-XX.Y <título corto>

<descripción detallada>
- punto 1
- punto 2

Tests:
- N unit tests
- N integration tests
- Sabotaje test K cubre invariante X

Refs: HU-XX.Y
```

### Step 11: Actualizar state.yaml

```yaml
# openspec/changes/REQ-XX/HU-XX.Y/state.yaml
status: implemented   # antes: proposed
created: 2026-06-07
implemented: 2026-06-08
archived: ~
```

### Step 12: Actualizar CHANGELOG.md

Sección `[Unreleased]` con bullet apropiado (Added/Fixed/Changed/etc.).

## Setup testcontainers (HU-01.1 ya lo usa)

```go
//go:build integration

package store_test

import (
    "context"
    "testing"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/stretchr/testify/require"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func setupPostgres(t *testing.T) (*pgxpool.Pool, func()) {
    t.Helper()
    ctx := context.Background()
    pgC, err := postgres.Run(ctx, "pgvector/pgvector:pg16",
        postgres.WithDatabase("test"),
        postgres.WithUsername("test"),
        postgres.WithPassword("test"),
    )
    require.NoError(t, err)
    dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
    pool, err := pgxpool.New(ctx, dsn)
    require.NoError(t, err)
    cleanup := func() {
        pool.Close()
        _ = pgC.Terminate(ctx)
    }
    return pool, cleanup
}
```

## Verificación end-to-end por HU

Cada HU termina con un smoke manual documentado en su `tasks.md` sección Cierre. Ejemplo HU-01.1:

```bash
make dev-up
make dev-migrate
./bin/domain migrate version  # → schema version: 23 (dirty=false)
./bin/domain migrate down -1
./bin/domain migrate version  # → 22
./bin/domain migrate up
```

## Anti-patterns prohibidos

- ❌ Implementar antes de escribir test (TDD inverso)
- ❌ Test que solo verifica el mock, no la lógica
- ❌ Skipear sabotaje porque "es obvio"
- ❌ Coverage <70% mergeado sin justificación
- ❌ Test integration que NO usa testcontainers (toca prod o local DB sucia)
- ❌ Tests con `time.Sleep` en lugar de retry-until-condition
- ❌ Test que falla intermitente (flaky) sin investigar root cause
- ❌ Tests interdependientes (orden importa)

## CI gating (HU-19.1)

```yaml
jobs:
  lint:        # golangci-lint + custom linters (HU-25.13, HU-13.9)
  unit:        # go test -race -short ./...
  integration: # go test -race -tags=integration ./... con DinD
  coverage:    # codecov + PR comment + threshold check
```

Branch protection `main`: required checks `lint, unit, integration`. PR sin tests → fail.

## Métricas de calidad por HU implementada

| dimensión | target |
|-----------|--------|
| Cobertura escenarios Gherkin → tests | 100% |
| Cobertura tipos error tipados → tests | 100% |
| Sabotaje tests | ≥1 por HU |
| Unit + integration ratio | >70% unit, <30% integration |
| Tiempo total tests HU | <30s unit, <2min integration |
| Flaky rate | 0% (3 corridas consecutivas mismas) |
