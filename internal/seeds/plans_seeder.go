package seeds

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// PlansSeeder inserta planes por defecto (HU-01.7).
type PlansSeeder struct{}

func (s *PlansSeeder) Name() string   { return "plans" }
func (s *PlansSeeder) Version() int   { return 1 }
func (s *PlansSeeder) Order() int     { return 10 }
func (s *PlansSeeder) IsDevOnly() bool { return false }

func (s *PlansSeeder) Run(ctx context.Context, tx pgx.Tx, env Env) (Report, error) {
	var rep Report
	plans := []struct {
		Slug            string
		Name            string
		TokensPerMonth  int64
		RunsPerMonth    int64
		StorageGBMax    int64
		MembersMax      int
		Seats           int
		MonthlyPriceUSD int
	}{
		{"free", "Free", 100000, 100, 1, 3, 1, 0},
		{"starter", "Starter", 1000000, 1000, 10, 10, 3, 2900},
		{"team", "Team", 5000000, 5000, 50, 25, 10, 9900},
		{"enterprise", "Enterprise", 50000000, 50000, 500, 100, 50, 99900},
	}
	for _, p := range plans {
		tag, err := tx.Exec(ctx, `
			INSERT INTO plans (slug, name, tokens_per_month, runs_per_month,
			                   storage_gb_max, members_max, seats, monthly_price_usd)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT (slug) DO UPDATE
			SET name           = EXCLUDED.name,
			    tokens_per_month = EXCLUDED.tokens_per_month,
			    runs_per_month   = EXCLUDED.runs_per_month,
			    storage_gb_max   = EXCLUDED.storage_gb_max,
			    members_max      = EXCLUDED.members_max,
			    seats            = EXCLUDED.seats,
			    monthly_price_usd = EXCLUDED.monthly_price_usd
		`, p.Slug, p.Name, p.TokensPerMonth, p.RunsPerMonth,
			p.StorageGBMax, p.MembersMax, p.Seats, p.MonthlyPriceUSD)
		if err != nil {
			return rep, fmt.Errorf("seed plan %s: %w", p.Slug, err)
		}
		n := tag.RowsAffected()
		if n == 1 {
			rep.Created++
		} else {
			rep.Updated++
		}
	}
	return rep, nil
}
