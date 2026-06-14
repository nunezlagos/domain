# Design: issue-01.7-seeders-system

## Estructura

```
internal/seeds/
  seeder.go          # Interface + Registry + Run orchestrator
  embed.go           # go:embed catalogs
  catalogs/
    plans.go         # in-code seeds
    models.go        # in-code with YAML embed
    agents/
      code-reviewer.yaml
      architecture-advisor.yaml
      bug-hunter.yaml
      pr-reviewer.yaml
      doc-writer.yaml
    skills/
      summarize-text.yaml
      embed-text.yaml
      search-web.yaml
      ... (catalog inicial)
    flows/
      onboarding-org.yaml
      daily-summary.yaml
    notifications/
      otp_email.yaml
      invitation_email.yaml
      usage_alert.yaml
    policies/         # MD desde .claude/rules/ committed aquí también
      go.md
      sdd.md
      clean-architecture.md
      db.md
      api.md
      security.md
      testing.md
      observability.md
      migrations.md
    error_codes.yaml
    crons.go
  reports.go         # Report types
  cli.go             # `domain seed` subcomando
```

## Seeder interface

```go
type Seeder interface {
  Name() string
  Version() int            // bump cuando cambian catalog
  Order() int              // dependency order
  Run(ctx context.Context, tx pgx.Tx, env Env) (Report, error)
}

type Report struct {
  Created  int
  Updated  int
  Skipped  int  // already up-to-date
  Preserved int // user-modified, not overwritten
  Errors   []string
}
```

## Schema

```sql
CREATE TABLE seed_versions (
  seeder_name VARCHAR(100) PRIMARY KEY,
  applied_version INT NOT NULL,
  last_applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_report JSONB
);

-- Convención per tabla seedable:
ALTER TABLE agents
  ADD COLUMN seed_managed BOOLEAN DEFAULT false,
  ADD COLUMN seed_version INT,
  ADD COLUMN is_user_modified BOOLEAN DEFAULT false,
  ADD COLUMN updated_at_from_seed TIMESTAMPTZ;
```

## Advisory lock

```go
const seedLockID = int64(0xD0_A1_15_EE_DE_DE)  // arbitrary

func RunAll(ctx context.Context, pool *pgxpool.Pool, env Env) error {
  // try acquire lock
  var got bool
  pool.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", seedLockID).Scan(&got)
  if !got {
    slog.InfoContext(ctx, "another pod is seeding; skipping")
    return nil
  }
  defer pool.Exec(ctx, "SELECT pg_advisory_unlock($1)", seedLockID)
  
  for _, s := range Sorted(Registered()) {
    tx, _ := pool.Begin(ctx)
    rep, err := s.Run(ctx, tx, env)
    if err != nil { tx.Rollback(ctx); return err }
    tx.Commit(ctx)
    LogReport(s.Name(), rep)
  }
  return nil
}
```

## UPSERT pattern

```go
// per row
_, err := tx.Exec(ctx, `
  INSERT INTO agents (slug, name, system_prompt, ..., seed_managed, seed_version)
  VALUES ($1, $2, $3, ..., true, $V)
  ON CONFLICT (slug) DO UPDATE SET
    name = EXCLUDED.name,
    system_prompt = EXCLUDED.system_prompt,
    seed_version = EXCLUDED.seed_version,
    updated_at_from_seed = NOW()
  WHERE agents.is_user_modified = false  -- KEY: respeta user mod
`, slug, name, prompt, ...)
```

## CLI

```bash
domain seed --all                # all seeders
domain seed --only plans,models  # selective
domain seed --dry-run            # report sin tocar
domain seed --force              # ignora seed_version, re-aplica
```

## TDD plan

1. Empty DB + boot → seeds aplicados, report N rows
2. Re-boot → 0 changes
3. Bump version código → re-seed esa entrada
4. is_user_modified=true → respeta
5. Dev env → DevOnly también
6. Prod env → DevOnly skip
7. Advisory lock: 2 goroutines simultáneas → 1 ejecuta, otra skip
8. Dry-run no muta
9. YAML inválido en seed → fail-fast boot
10. Sabotaje: cambiar slug en YAML después de prod → error "slug renaming not allowed; rotate via migration"
