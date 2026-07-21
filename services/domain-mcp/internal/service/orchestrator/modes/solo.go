package modes

import (
	"nunezlagos/domain/internal/llm"
)

// ProviderForModel infiere el nombre del provider LLM desde el model name.
// Delega en llm.ProviderNameForModel (fuente única del prefix-map, DOMAINSERV-57).
// Evita persistir un campo provider duplicado en agent_templates (el model name
// ya identifica unívocamente el provider).
func ProviderForModel(model string) string {
	return llm.ProviderNameForModel(model)
}
