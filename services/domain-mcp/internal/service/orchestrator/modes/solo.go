package modes

import (
	"strings"
)

// ProviderForModel infiere el nombre del provider LLM desde el model
// name siguiendo la convención del catálogo:
//   - claude-*           → anthropic
//   - gpt-*, openai-*    → openai
//   - gemini-*, google-* → google
//   - minimax-*          → minimax (endpoint anthropic-compatible)
//   - resto              → ollama (default self-hosted)
//
// Esto evita persistir un campo provider duplicado en agent_templates
// (model name ya identifica unívocamente el provider).
func ProviderForModel(model string) string {
	m := strings.ToLower(model)
	switch {
	case strings.HasPrefix(m, "claude"):
		return "anthropic"
	case strings.HasPrefix(m, "gpt"), strings.HasPrefix(m, "openai"):
		return "openai"
	case strings.HasPrefix(m, "gemini"), strings.HasPrefix(m, "google"):
		return "google"
	case strings.HasPrefix(m, "minimax"):
		return "minimax"
	default:
		return "ollama"
	}
}
