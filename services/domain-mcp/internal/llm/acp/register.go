package acp

import (
	"context"
	"log/slog"
	"os"
	"strings"

	acpbridge "nunezlagos/domain/internal/agentbridge/acp"
	"nunezlagos/domain/internal/llm"
)

const (
	// ProviderName es el nombre registrado en el llm.Factory
	ProviderName = "acp"
	// DisabledEnv apaga el registro del provider ACP (por default está ON)
	DisabledEnv = "DOMAIN_ACP_DISABLED"
)

// New construye el provider ACP que spawnea opencode por cada Complete. Si hay
// modelos free descubribles, activa el rolling (rota entre ellos por Complete).
func New(cfg acpbridge.Config, logger *slog.Logger) *Provider {
	p := &Provider{
		name: ProviderName,
		spawn: func(ctx context.Context) (runner, error) {
			pr, err := acpbridge.Spawn(ctx, cfg, logger)
			if err != nil {
				return nil, err
			}
			return pr, nil
		},
	}
	if roll := buildRoller(context.Background()); roll != nil {
		p.roll = roll
		p.spawnHome = func(ctx context.Context, home string) (runner, error) {
			c := cfg
			c.Env = append(append([]string{}, cfg.Env...), "HOME="+home)
			pr, err := acpbridge.Spawn(ctx, c, logger)
			if err != nil {
				return nil, err
			}
			return pr, nil
		}
	}
	return p
}

// buildRoller arma el roller con el roster descubierto (o el override por env).
// Devuelve nil si no hay modelos free (cae al path legacy de un solo modelo).
func buildRoller(ctx context.Context) *roller {
	base, err := os.MkdirTemp("", "acp-model-homes")
	if err != nil {
		return nil
	}
	r := newRoller(base, defaultCooldown, discoverFreeModels)
	if env := os.Getenv("DOMAIN_ACP_FREE_MODELS"); env != "" {
		r.setRoster(splitModels(env))
	} else {
		r.refreshNow(ctx)
	}
	if r.size() == 0 {
		return nil
	}
	go r.refreshLoop(ctx, defaultRefreshTTL)
	return r
}

func splitModels(csv string) []string {
	var out []string
	for _, m := range strings.Split(csv, ",") {
		if s := strings.TrimSpace(m); s != "" {
			out = append(out, s)
		}
	}
	return out
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
