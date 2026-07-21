package llm

import (
	"os"
	"strings"
)

// Role identifica una función IA que resuelve a un {provider, model} por config.
// Las funciones piden su rol; no referencian un provider/modelo literal.
type Role string

const (
	RoleRerank      Role = "rerank"
	RoleInfer       Role = "infer"
	RoleClassify    Role = "classify"
	RoleJudge       Role = "judge"
	RoleAgent       Role = "agent"
	RoleOrchestrate Role = "orchestrate"
	RoleEmbed       Role = "embed"
)

// roleBinding es el default {provider, model} de un rol cuando no hay override.
type roleBinding struct {
	provider string
	model    string
}

// defaultRoleBindings centraliza los defaults que antes vivían hardcodeados en
// cada call-site. Overridables por env DOMAIN_LLM_<ROLE>_{PROVIDER,MODEL}.
var defaultRoleBindings = map[Role]roleBinding{
	RoleRerank:   {provider: "minimax", model: "MiniMax-M3"},
	RoleInfer:    {provider: "minimax", model: "MiniMax-M3"},
	RoleJudge:    {provider: "minimax", model: "MiniMax-M3"},
	RoleClassify: {provider: "anthropic", model: "claude-haiku-4-5-20251001"},
}

// ProviderNameForModel deriva el nombre de provider registrado desde el prefijo
// del modelo. Fuente única del mapeo (modes.ProviderForModel delega aquí).
func ProviderNameForModel(model string) string {
	switch m := strings.ToLower(model); {
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

// RoleModel resuelve el modelo efectivo de un rol: env DOMAIN_LLM_<ROLE>_MODEL,
// si no el default del rol (puede ser "").
func RoleModel(role Role) string {
	if v := strings.TrimSpace(os.Getenv(roleEnv(role, "MODEL"))); v != "" {
		return v
	}
	return defaultRoleBindings[role].model
}

// ProviderForRole resuelve el {provider, model} de un rol contra el factory:
//  1. provider = env DOMAIN_LLM_<ROLE>_PROVIDER; si no, el default del rol; si
//     no, el derivado del modelo.
//  2. si ese provider no está registrado, cae al provider primario del factory
//     (GetDefault) con model="" (el provider usa su modelo por default).
func (f *Factory) ProviderForRole(role Role) (Provider, string, error) {
	model := RoleModel(role)
	name := strings.TrimSpace(os.Getenv(roleEnv(role, "PROVIDER")))
	if name == "" {
		name = defaultRoleBindings[role].provider
	}
	if name == "" {
		name = ProviderNameForModel(model)
	}
	if p, err := f.Get(name); err == nil {
		return p, model, nil
	}
	p, err := f.GetDefault()
	if err != nil {
		return nil, "", err
	}
	return p, "", nil
}

func roleEnv(role Role, suffix string) string {
	return "DOMAIN_LLM_" + strings.ToUpper(string(role)) + "_" + suffix
}
