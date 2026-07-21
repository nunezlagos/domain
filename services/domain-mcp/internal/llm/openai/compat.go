package openai

import (
	"encoding/json"
	"log/slog"
	"os"

	"nunezlagos/domain/internal/llm"
)

// CompatProvidersEnv es la env con la lista JSON de endpoints OpenAI-compatibles.
const CompatProvidersEnv = "DOMAIN_OPENAI_COMPAT_PROVIDERS"

// compatConfig describe un endpoint OpenAI-compatible configurado por env.
// APIKeyEnv es el NOMBRE de la env que contiene la key, nunca la key misma.
type compatConfig struct {
	Name      string `json:"name"`
	BaseURL   string `json:"base_url"`
	APIKeyEnv string `json:"api_key_env"`
	Model     string `json:"model"`
}

// RegisterOpenAICompat registra 1..N providers OpenAI-compatibles leídos de
// DOMAIN_OPENAI_COMPAT_PROVIDERS (JSON array), cada uno reusando el dialecto
// openai vía NewWithBaseURL. Parse tolerante: JSON inválido o item sin
// name/base_url se skipea con warning (sin exponer la key) sin abortar el resto.
// wrap envuelve cada provider (retry/ratelimit/circuitbreaker). Devuelve la
// cantidad registrada.
func RegisterOpenAICompat(factory *llm.Factory, wrap func(llm.Provider) llm.Provider, logger *slog.Logger) int {
	raw := os.Getenv(CompatProvidersEnv)
	if raw == "" {
		return 0
	}
	var cfgs []compatConfig
	if err := json.Unmarshal([]byte(raw), &cfgs); err != nil {
		warn(logger, "openai-compat: JSON inválido, ignorando", "env", CompatProvidersEnv, "error", err.Error())
		return 0
	}
	n := 0
	for _, c := range cfgs {
		if c.Name == "" || c.BaseURL == "" {
			warn(logger, "openai-compat: item sin name o base_url, skip", "name", c.Name, "base_url", c.BaseURL)
			continue
		}
		p := NewWithBaseURL(os.Getenv(c.APIKeyEnv), c.BaseURL, c.Model)
		if wrap != nil {
			factory.Register(c.Name, wrap(p))
		} else {
			factory.Register(c.Name, p)
		}
		info(logger, "openai-compat provider registrado", "name", c.Name, "base_url", c.BaseURL, "model", c.Model)
		n++
	}
	return n
}

func warn(logger *slog.Logger, msg string, args ...any) {
	if logger != nil {
		logger.Warn(msg, args...)
	}
}

func info(logger *slog.Logger, msg string, args ...any) {
	if logger != nil {
		logger.Info(msg, args...)
	}
}
