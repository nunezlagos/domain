package acp

import (
	"context"
	"log/slog"
	"os"

	acpbridge "nunezlagos/domain/internal/agentbridge/acp"
	"nunezlagos/domain/internal/llm"
)

const (
	// ProviderName es el nombre registrado en el llm.Factory
	ProviderName = "acp"
	// DisabledEnv apaga el registro del provider ACP (por default está ON)
	DisabledEnv = "DOMAIN_ACP_DISABLED"
)

// New construye el provider ACP que spawnea opencode por cada Complete
func New(cfg acpbridge.Config, logger *slog.Logger) *Provider {
	return &Provider{
		name: ProviderName,
		spawn: func(ctx context.Context) (runner, error) {
			p, err := acpbridge.Spawn(ctx, cfg, logger)
			if err != nil {
				return nil, err
			}
			return p, nil
		},
	}
}

// Register registra el provider ACP en el factory y, salvo que haya un
// DOMAIN_LLM_PROVIDER explícito, lo deja como default (ACP ON por defecto).
// Devuelve false si está deshabilitado por env
func Register(factory *llm.Factory, wrap func(llm.Provider) llm.Provider, cfg acpbridge.Config, logger *slog.Logger) bool {
	if os.Getenv(DisabledEnv) != "" {
		return false
	}
	var p llm.Provider = New(cfg, logger)
	if wrap != nil {
		p = wrap(p)
	}
	factory.Register(ProviderName, p)
	if os.Getenv("DOMAIN_LLM_PROVIDER") == "" {
		factory.SetDefault(ProviderName, "")
	}
	return true
}
