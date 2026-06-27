package anthropic

import (
	"os"

	"nunezlagos/domain/internal/llm"
)

// MiniMax wiring — reusa el provider anthropic apuntando al endpoint
// anthropic-compatible de MiniMax. NO requiere un provider nuevo: MiniMax
// expone /v1/messages con los mismos headers (x-api-key + anthropic-version)
// que la API oficial de Anthropic.
const (
	// MiniMaxProviderName es el nombre bajo el cual se registra en el Factory
	// y al que resuelve ProviderForModel("MiniMax-M3").
	MiniMaxProviderName = "minimax"

	// MiniMaxModel es el nombre de modelo enviado tal cual en el body (case-sensitive).
	MiniMaxModel = "MiniMax-M3"

	// MiniMaxBaseURL es el endpoint internacional anthropic-compatible.
	// Si en el futuro se necesita China, parametrizar con MINIMAX_REGION ->
	// https://api.minimaxi.com/anthropic.
	MiniMaxBaseURL = "https://api.minimax.io/anthropic"
)

// MiniMaxAPIKey resuelve la API key de MiniMax desde el entorno.
//
// Preferencia: MINIMAX_API_KEY (misma key que consume el Django domain-admin,
// comparten VPS — única excepción deliberada a la convención DOMAIN_*).
// Fallback: DOMAIN_MINIMAX_API_KEY para quien prefiera mantener el prefijo.
// Devuelve "" si ninguna está seteada (degradación: el provider no se registra).
func MiniMaxAPIKey() string {
	if k := os.Getenv("MINIMAX_API_KEY"); k != "" {
		return k
	}
	return os.Getenv("DOMAIN_MINIMAX_API_KEY")
}

// MiniMaxAvailable indica si MiniMax está configurado (hay key en el entorno).
// Los consumidores (rerank de memorias, inferencia de aristas) pueden usar esto
// para decidir si intentar la llamada o degradar, sin depender de un Factory.Get
// fallido. Nota: la verificación canónica sigue siendo Factory.Get("minimax"),
// pero este helper permite chequear disponibilidad sin tener el Factory a mano.
func MiniMaxAvailable() bool {
	return MiniMaxAPIKey() != ""
}

// RegisterMiniMax registra el provider 'minimax' en el Factory si hay key.
// wrap permite envolver el provider con retry/ratelimit/circuitbreaker según
// el sitio de wiring (HTTP server vs MCP stdio). Devuelve true si se registró.
func RegisterMiniMax(factory *llm.Factory, wrap func(llm.Provider) llm.Provider) bool {
	k := MiniMaxAPIKey()
	if k == "" {
		return false
	}
	p := NewWithBaseURL(k, MiniMaxBaseURL, MiniMaxModel)
	if wrap != nil {
		factory.Register(MiniMaxProviderName, wrap(p))
	} else {
		factory.Register(MiniMaxProviderName, p)
	}
	return true
}
