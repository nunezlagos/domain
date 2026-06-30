package seeds

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"

	"nunezlagos/domain/internal/observability"
)

// KnownErrorsSeeder siembra los known_errors canonicos (issue-53.9). La data
// vive en observability.KnownErrorSeeds(); aca solo se persiste idempotente.
type KnownErrorsSeeder struct{}

func (s *KnownErrorsSeeder) Name() string    { return "known_errors" }
func (s *KnownErrorsSeeder) Version() int    { return 1 }
func (s *KnownErrorsSeeder) Order() int      { return 45 }
func (s *KnownErrorsSeeder) IsDevOnly() bool { return false }

// Run hace upsert idempotente (ON CONFLICT DO NOTHING) de cada known_error.
func (s *KnownErrorsSeeder) Run(ctx context.Context, tx pgx.Tx, _ Env) (Report, error) {
	var rep Report
	for _, ke := range observability.KnownErrorSeeds() {
		var params []byte
		if ke.ActionParams != nil {
			params, _ = json.Marshal(ke.ActionParams)
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO known_errors
				(fingerprint, name, recoverable, auto_heal_action, action_params)
			VALUES ($1,$2,$3,$4,$5)
			ON CONFLICT (fingerprint) DO NOTHING
		`, ke.Fingerprint, ke.Name, ke.Recoverable, ke.AutoHealAction, params)
		if err != nil {
			return rep, fmt.Errorf("seed known_error %s: %w", ke.Name, err)
		}
		if tag.RowsAffected() == 1 {
			rep.Created++
		} else {
			rep.Skipped++
		}
	}
	return rep, nil
}
