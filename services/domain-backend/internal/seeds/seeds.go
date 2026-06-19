// Package seeds — issue-01.7 seeders-system.
//
// Framework Go de seeders idempotente con go:embed. Cada Seeder:
//   - tiene Name único + Version int
//   - Run(ctx, tx, env) ejecuta UPSERT idempotente
//   - reporta Report con counts (created/updated/skipped/preserved/errors)
//
// Orchestrator usa advisory lock Postgres para safe concurrent boot (N pods).
package seeds

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Env env target del seed.
type Env string

const (
	EnvDev     Env = "dev"
	EnvStaging Env = "staging"
	EnvProd    Env = "prod"
)

// Report resultado de un seeder run.
type Report struct {
	Created   int      `json:"created"`
	Updated   int      `json:"updated"`
	Skipped   int      `json:"skipped"`
	Preserved int      `json:"preserved"`         // user-modified, no sobrescrito
	Deleted   int      `json:"deleted,omitempty"` // cleanup defensivo (issue-08.10)
	Errors    []string `json:"errors,omitempty"`
}

// Seeder interface a implementar por cada catálogo.
type Seeder interface {
	// Name único, snake_case (e.g. "model_registry", "agent_templates").
	Name() string
	// Version se bumpea cuando el catalog cambia para forzar reseed.
	Version() int
	// Order para topological sort (model_registry antes que invitations, etc.). Lower first.
	Order() int
	// IsDevOnly true si solo corre en EnvDev (e.g. demo data).
	IsDevOnly() bool
	// Run aplica el seed idempotente con UPSERT pattern.
	Run(ctx context.Context, tx pgx.Tx, env Env) (Report, error)
}

// Registry mantiene seeders registrados.
type Registry struct {
	seeders []Seeder
}

// NewRegistry crea registry vacío.
func NewRegistry() *Registry { return &Registry{} }

// Register agrega seeder al registry.
func (r *Registry) Register(s Seeder) {
	r.seeders = append(r.seeders, s)
}

// Sorted retorna seeders ordenados por Order asc + name secondary.
func (r *Registry) Sorted() []Seeder {
	out := make([]Seeder, len(r.seeders))
	copy(out, r.seeders)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Order() != out[j].Order() {
			return out[i].Order() < out[j].Order()
		}
		return out[i].Name() < out[j].Name()
	})
	return out
}

// Names lista nombres registrados.
func (r *Registry) Names() []string {
	names := make([]string, len(r.seeders))
	for i, s := range r.seeders {
		names[i] = s.Name()
	}
	sort.Strings(names)
	return names
}

// Find retorna seeder por name o nil.
func (r *Registry) Find(name string) Seeder {
	for _, s := range r.seeders {
		if s.Name() == name {
			return s
		}
	}
	return nil
}

const (
	// seedLockID arbitrary BIGINT para advisory lock global de seed.
	seedLockID = int64(0x73656564) // "seed"
)

// RunAll ejecuta todos los seeders aplicables al env, con advisory lock.
// Si another pod tiene el lock, retorna nil sin ejecutar (otro está seedeando).
func (r *Registry) RunAll(ctx context.Context, pool *pgxpool.Pool, env Env) (map[string]Report, error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	// Try advisory lock; si no lo obtiene, skip silencioso.
	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", seedLockID).Scan(&locked); err != nil {
		return nil, fmt.Errorf("advisory lock: %w", err)
	}
	if !locked {
		return nil, errors.New("another seed run in progress")
	}
	defer func() {
		_, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", seedLockID)
	}()

	results := map[string]Report{}
	for _, s := range r.Sorted() {
		if s.IsDevOnly() && env != EnvDev {
			results[s.Name()] = Report{Skipped: 1}
			continue
		}
		// Check if already at this version (idempotency fast-path)
		var appliedVersion int
		err := conn.QueryRow(ctx,
			`SELECT applied_version FROM seed_versions WHERE seeder_name = $1`,
			s.Name(),
		).Scan(&appliedVersion)
		alreadyApplied := err == nil && appliedVersion >= s.Version()
		if alreadyApplied {
			results[s.Name()] = Report{Skipped: 1}
			continue
		}

		// tx
		tx, err := conn.Begin(ctx)
		if err != nil {
			return results, fmt.Errorf("begin tx for %s: %w", s.Name(), err)
		}
		rep, runErr := s.Run(ctx, tx, env)
		if runErr != nil {
			_ = tx.Rollback(ctx)
			rep.Errors = append(rep.Errors, runErr.Error())
			results[s.Name()] = rep
			return results, fmt.Errorf("seeder %s failed: %w", s.Name(), runErr)
		}

		// upsert seed_versions
		repJSON, _ := json.Marshal(rep)
		_, err = tx.Exec(ctx, `
			INSERT INTO seed_versions (seeder_name, applied_version, last_applied_at, last_report)
			VALUES ($1, $2, NOW(), $3)
			ON CONFLICT (seeder_name) DO UPDATE
			SET applied_version = EXCLUDED.applied_version,
			    last_applied_at = NOW(),
			    last_report = EXCLUDED.last_report
		`, s.Name(), s.Version(), repJSON)
		if err != nil {
			_ = tx.Rollback(ctx)
			return results, fmt.Errorf("record seed_version %s: %w", s.Name(), err)
		}
		if err := tx.Commit(ctx); err != nil {
			return results, fmt.Errorf("commit %s: %w", s.Name(), err)
		}
		results[s.Name()] = rep
	}
	return results, nil
}

// AppliedVersion lookup de version aplicada para un seeder.
func AppliedVersion(ctx context.Context, pool *pgxpool.Pool, name string) (int, bool, error) {
	var v int
	err := pool.QueryRow(ctx,
		`SELECT applied_version FROM seed_versions WHERE seeder_name = $1`,
		name,
	).Scan(&v)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("query applied version: %w", err)
	}
	return v, true, nil
}
