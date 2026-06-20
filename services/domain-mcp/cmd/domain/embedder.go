package main

import (
	"log/slog"
	"os"
	"strings"

	"nunezlagos/domain/internal/llm"
	llmopenai "nunezlagos/domain/internal/llm/openai"
)

// chooseEmbedder elige el embedder según env. REQ-68.
//
//   DOMAIN_EMBEDDING_PROVIDER=noop     → NopEmbedder (default; vector zero)
//   DOMAIN_EMBEDDING_PROVIDER=openai   → openai.Embedder con DOMAIN_OPENAI_API_KEY
//   DOMAIN_EMBEDDING_PROVIDER=fake     → FakeEmbedder (determinístico,
//                                        solo para tests E2E)
//
// Si openai está pedido pero falta la key → log.Warn + NopEmbedder.
// Se prefiere arrancar el server con búsqueda semántica desactivada
// antes de fallar todo el wireup por un secret faltante.
func chooseEmbedder(logger *slog.Logger) llm.Embedder {
	if logger == nil {
		logger = slog.Default()
	}
	kind := strings.ToLower(strings.TrimSpace(os.Getenv("DOMAIN_EMBEDDING_PROVIDER")))
	switch kind {
	case "", "noop":
		logger.Info("embedder: noop (búsqueda semántica desactivada; RAG cae a FTS)")
		return llm.NopEmbedder{}
	case "fake":
		logger.Warn("embedder: fake (determinístico, NO usar en prod)")
		return llm.FakeEmbedder{}
	case "openai":
		key := strings.TrimSpace(os.Getenv("DOMAIN_OPENAI_API_KEY"))
		if key == "" {
			logger.Warn("DOMAIN_EMBEDDING_PROVIDER=openai pero DOMAIN_OPENAI_API_KEY vacío; usando noop")
			return llm.NopEmbedder{}
		}
		model := strings.TrimSpace(os.Getenv("DOMAIN_OPENAI_EMBED_MODEL"))
		if model == "" {
			model = "text-embedding-3-small"
		}
		logger.Info("embedder: openai", slog.String("model", model))
		return llmopenai.NewEmbedder(llmopenai.EmbedderConfig{
			APIKey: key,
			Model:  model,
		})
	default:
		logger.Warn("DOMAIN_EMBEDDING_PROVIDER desconocido; usando noop",
			slog.String("provider", kind))
		return llm.NopEmbedder{}
	}
}
