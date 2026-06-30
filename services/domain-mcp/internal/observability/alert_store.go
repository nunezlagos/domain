// Package observability: este archivo cubre la carga de AlertConfig desde la
// tabla alert_configs para alimentar el AlertEngine.
//
// issue-53.9 early-error-reporting.
package observability

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// PGAlertConfigStore carga los alert_configs habilitados. Pool nullable; setear via SetPool.
type PGAlertConfigStore struct {
	Pool *pgxpool.Pool
}

// SetPool setea el pool (post-init).
func (s *PGAlertConfigStore) SetPool(p *pgxpool.Pool) { s.Pool = p }

// LoadConfigs devuelve los alert_configs con enabled=true.
func (s *PGAlertConfigStore) LoadConfigs(ctx context.Context) ([]AlertConfig, error) {
	if s.Pool == nil {
		return nil, ErrStoreNotReady
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT category, severity_min, channel, channel_config, throttle_seconds
		FROM alert_configs
		WHERE enabled
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AlertConfig
	for rows.Next() {
		var (
			cfg    AlertConfig
			cat    string
			params []byte
		)
		if err := rows.Scan(&cat, &cfg.SeverityMin, &cfg.Channel, &params, &cfg.ThrottleSeconds); err != nil {
			return nil, err
		}
		cfg.Category = Category(cat)
		if len(params) > 0 {
			if err := json.Unmarshal(params, &cfg.ChannelConfig); err != nil {
				return nil, fmt.Errorf("decode channel_config (%s/%s): %w", cat, cfg.Channel, err)
			}
		}
		out = append(out, cfg)
	}
	return out, rows.Err()
}
