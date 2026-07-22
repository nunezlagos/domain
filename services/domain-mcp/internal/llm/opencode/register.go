package opencode

import (
	"log/slog"
	"os"

	"nunezlagos/domain/internal/llm"
)

const (
	// URLEnv habilita el provider: base URL del sidecar opencode serve
	URLEnv  = "DOMAIN_OPENCODE_URL"
	userEnv = "OPENCODE_SERVER_USERNAME"
	passEnv = "OPENCODE_SERVER_PASSWORD"
)

// Register registra el provider opencode-HTTP si DOMAIN_OPENCODE_URL está
// seteada y lo deja como default (cerebro de prod), salvo DOMAIN_LLM_PROVIDER
// explícito. Sin URL devuelve false y no toca el factory
func Register(factory *llm.Factory, wrap func(llm.Provider) llm.Provider, logger *slog.Logger) bool {
	url := os.Getenv(URLEnv)
	if url == "" {
		return false
	}
	var p llm.Provider = New(url, nil, os.Getenv(userEnv), os.Getenv(passEnv))
	if wrap != nil {
		p = wrap(p)
	}
	factory.Register(ProviderName, p)
	if os.Getenv("DOMAIN_LLM_PROVIDER") == "" {
		factory.SetDefault(ProviderName, "")
	}
	if logger != nil {
		logger.Info("opencode provider registrado", slog.String("url", url))
	}
	return true
}
