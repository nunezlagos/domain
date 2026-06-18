package seeds

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// PlansSeeder siembra los TIERS DE USO INTERNOS — issue-01.7 + issue-21.3.
//
// IMPORTANTE: Domain es open-source SIN COBRO (decisión user 2026-06-08;
// issue-21.4 stripe-billing archivada). Estos planes NO son tiers comerciales:
// son perfiles de control de uso interno (quotas + RBAC) que el operador
// del despliegue asigna a cada org según su criterio (gratis, paga aparte,
// trial, lo que sea — Domain no maneja la facturación).
//
// monthly_price_usd queda en 0 deliberadamente — la columna existe en el
// schema (000032) pero no se usa para nada en código. Si en el futuro
// alguien forkea Domain y agrega billing, podrá poblar este campo desde
// su propio seeder. Acá NO.
type PlansSeeder struct{}

func (s *PlansSeeder) Name() string    { return "plans" }
func (s *PlansSeeder) Version() int    { return 2 } // bump por cambio semántico
func (s *PlansSeeder) Order() int      { return 10 }
func (s *PlansSeeder) IsDevOnly() bool { return false }

func (s *PlansSeeder) Run(ctx context.Context, tx pgx.Tx, env Env) (Report, error) {
	var rep Report
	// Tiers nombrados por nivel de uso, NO por pricing. Cada uno es un
	// preset de quotas que el operador asigna a la org via service/billing
	// (que pese al nombre legacy, no factura — solo controla uso).
	plans := []struct {
		Slug           string
		Name           string
		TokensPerMonth int64
		RunsPerMonth   int64
		StorageGBMax   int64
		MembersMax     int
		Seats          int
	}{
		// trial: defaults conservadores para evaluación/dev local
		{"trial", "Trial (uso evaluación)", 100_000, 100, 1, 3, 1},
		// standard: org chica self-hosted típica
		{"standard", "Standard (uso productivo chico)", 1_000_000, 1_000, 10, 10, 3},
		// extended: equipos medianos, varios proyectos
		{"extended", "Extended (uso productivo extendido)", 5_000_000, 5_000, 50, 25, 10},
		// unlimited: sin caps prácticos para self-hosted sin fricción
		{"unlimited", "Unlimited (sin caps prácticos)", 50_000_000, 50_000, 500, 100, 50},
	}
	for _, p := range plans {
		tag, err := tx.Exec(ctx, `
			INSERT INTO plans (slug, name, tokens_per_month, runs_per_month,
			                   storage_gb_max, members_max, seats, monthly_price_usd)
			VALUES ($1,$2,$3,$4,$5,$6,$7, 0)
			ON CONFLICT (slug) DO UPDATE
			SET name              = EXCLUDED.name,
			    tokens_per_month  = EXCLUDED.tokens_per_month,
			    runs_per_month    = EXCLUDED.runs_per_month,
			    storage_gb_max    = EXCLUDED.storage_gb_max,
			    members_max       = EXCLUDED.members_max,
			    seats             = EXCLUDED.seats,
			    monthly_price_usd = 0
		`, p.Slug, p.Name, p.TokensPerMonth, p.RunsPerMonth,
			p.StorageGBMax, p.MembersMax, p.Seats)
		if err != nil {
			return rep, fmt.Errorf("seed plan %s: %w", p.Slug, err)
		}
		if tag.RowsAffected() == 1 {
			rep.Created++
		} else {
			rep.Updated++
		}
	}

	// Limpiar tiers viejos con nombres comerciales (free/pro/team/enterprise/
	// starter). Vienen tanto de la migration 000032 como del seeder v1.
	// En single-org (ISSUE-21.6) NO se borra nada: los plans son globales
	// y la columna organizations.plan_id se dropea en Fase C. El cleanup
	// defensivo queda desactivado (se reactivaría en multi-tenant).
	_ = cleanupSlugs // keep variable referenced for future multi-tenant reactivation
	// cleanupSlugs := []string{"free", "pro", "starter", "team", "enterprise"}
	// for _, slug := range cleanupSlugs {
	//   _, err := tx.Exec(ctx, `
	//       DELETE FROM plans WHERE slug = $1`, slug) // single-org: sin scope per-org
	//   ...
	// }
			rep.Errors = append(rep.Errors, fmt.Sprintf("cleanup %s: %v", slug, err))
		}
	}

	return rep, nil
}
