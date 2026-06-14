// Package migrate wrappea golang-migrate embebido en el binario.
// issue-01.1 db-schema-migrations.
//
// Migraciones SQL viven en internal/migrate/migrations/ embebidas con go:embed.
// Por convención .claude/rules/db.md, migraciones nunca renombrar ni reordenar.
package migrate

import (
	"embed"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// translateURL convierte postgres:// → pgx5:// para usar driver pgx v5.
func translateURL(u string) string {
	if strings.HasPrefix(u, "postgres://") {
		return "pgx5://" + strings.TrimPrefix(u, "postgres://")
	}
	if strings.HasPrefix(u, "postgresql://") {
		return "pgx5://" + strings.TrimPrefix(u, "postgresql://")
	}
	return u
}

//go:embed all:migrations
var migrationsFS embed.FS

// FS expone el filesystem embebido (útil para tests).
func FS() embed.FS { return migrationsFS }

func open(databaseURL string) (*migrate.Migrate, error) {
	d, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("iofs source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", d, translateURL(databaseURL))
	if err != nil {
		return nil, fmt.Errorf("migrate instance: %w", err)
	}
	return m, nil
}

// Up aplica todas las migraciones pendientes.
func Up(databaseURL string) error {
	m, err := open(databaseURL)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	return nil
}

// Down rollback. steps<0 = all.
func Down(databaseURL string, steps int) error {
	m, err := open(databaseURL)
	if err != nil {
		return err
	}
	defer m.Close()
	if steps < 0 {
		if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("migrate down all: %w", err)
		}
		return nil
	}
	if err := m.Steps(-steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate down %d: %w", steps, err)
	}
	return nil
}

// Version (version, dirty, error).
func Version(databaseURL string) (uint, bool, error) {
	m, err := open(databaseURL)
	if err != nil {
		return 0, false, err
	}
	defer m.Close()
	v, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, err
	}
	return v, dirty, nil
}
