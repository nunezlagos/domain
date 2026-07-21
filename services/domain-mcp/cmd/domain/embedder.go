package main

import (
	"log/slog"
	"os"
	"strings"

	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/llm/anthropic"
	"nunezlagos/domain/internal/llm/ollama"
	llmopenai "nunezlagos/domain/internal/llm/openai"
)

// embeddingDim es la dimensión del esquema pgvector (vector(1536) en todas las
// columnas de embeddings: observations, knowledge_docs, skills, semantic_cache).
// Un embedder con otra dimensión corrompería el índice → se degrada a noop.
const embeddingDim = 1536

// chooseEmbedder elige el embedder según env y valida su dimensión contra el
// esquema pgvector. REQ-68 / DOMAINSERV-60.
//
//	DOMAIN_EMBEDDING_PROVIDER=noop     → NopEmbedder (default; búsqueda semántica off, RAG cae a FTS)
//	DOMAIN_EMBEDDING_PROVIDER=openai   → openai.Embedder (DOMAIN_OPENAI_API_KEY, DOMAIN_OPENAI_EMBED_MODEL, DOMAIN_OPENAI_EMBED_BASE_URL)
//	DOMAIN_EMBEDDING_PROVIDER=voyage   → anthropic.VoyageEmbedder (DOMAIN_VOYAGE_API_KEY, DOMAIN_VOYAGE_EMBED_MODEL)
//	DOMAIN_EMBEDDING_PROVIDER=ollama   → ollama.Embedder (DOMAIN_OLLAMA_EMBED_MODEL, DOMAIN_OLLAMA_URL)
//	DOMAIN_EMBEDDING_PROVIDER=fake     → FakeEmbedder (determinístico, solo tests E2E)
//
// Si el provider pedido no tiene su key → log.Warn + NopEmbedder (arranca con
// búsqueda semántica desactivada antes de fallar el wireup por un secret).
func chooseEmbedder(logger *slog.Logger) llm.Embedder {
	if logger == nil {
		logger = slog.Default()
	}
	return validateDim(buildEmbedder(logger), logger)
}

func buildEmbedder(logger *slog.Logger) llm.Embedder {
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
		model := envOr("DOMAIN_OPENAI_EMBED_MODEL", "text-embedding-3-small")
		logger.Info("embedder: openai", slog.String("model", model))
		return llmopenai.NewEmbedder(llmopenai.EmbedderConfig{
			APIKey:  key,
			Model:   model,
			BaseURL: strings.TrimSpace(os.Getenv("DOMAIN_OPENAI_EMBED_BASE_URL")),
		})
	case "voyage":
		key := strings.TrimSpace(os.Getenv("DOMAIN_VOYAGE_API_KEY"))
		if key == "" {
			logger.Warn("DOMAIN_EMBEDDING_PROVIDER=voyage pero DOMAIN_VOYAGE_API_KEY vacío; usando noop")
			return llm.NopEmbedder{}
		}
		model := envOr("DOMAIN_VOYAGE_EMBED_MODEL", "voyage-3")
		logger.Info("embedder: voyage", slog.String("model", model))
		return anthropic.NewEmbedder(anthropic.EmbedderConfig{APIKey: key, Model: model})
	case "ollama":
		e := ollama.NewEmbedder(strings.TrimSpace(os.Getenv("DOMAIN_OLLAMA_EMBED_MODEL")))
		if base := strings.TrimSpace(os.Getenv("DOMAIN_OLLAMA_URL")); base != "" {
			e.BaseURL = base
		}
		logger.Info("embedder: ollama", slog.String("model", e.Model), slog.String("base_url", e.BaseURL))
		return e
	default:
		logger.Warn("DOMAIN_EMBEDDING_PROVIDER desconocido; usando noop", slog.String("provider", kind))
		return llm.NopEmbedder{}
	}
}

// validateDim degrada a noop si la dimensión del embedder no coincide con el
// esquema pgvector: escribir vectores de otra dimensión corrompería el índice.
// El mismatch se detecta al boot (observable) y no en runtime.
func validateDim(e llm.Embedder, logger *slog.Logger) llm.Embedder {
	if _, isNop := e.(llm.NopEmbedder); isNop {
		return e
	}
	if d := e.Dimensions(); d != embeddingDim {
		logger.Warn("embedder: dimensión incompatible con el esquema pgvector; degradando a noop",
			slog.Int("embedder_dim", d), slog.Int("schema_dim", embeddingDim))
		return llm.NopEmbedder{}
	}
	return e
}
