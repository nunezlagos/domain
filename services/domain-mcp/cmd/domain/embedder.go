package main

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/llm/anthropic"
	"nunezlagos/domain/internal/llm/ollama"
	llmopenai "nunezlagos/domain/internal/llm/openai"
)

// embeddingDim es la dimensión del esquema pgvector: knowledge_observations,
// knowledge_chunks, skills y chat_document_embeddings son vector(1024) desde
// la migración 000275 (bge-m3). Un embedder con otra dimensión rompería el
// INSERT → se degrada a noop.
const embeddingDim = 1024

// defaultProbeTimeout acota la medición de dimensión al arrancar: un provider
// lento o caído degrada a noop en vez de colgar el boot.
const defaultProbeTimeout = 10 * time.Second

// probeTimeout es var (no const) para que los tests no esperen el timeout real.
var probeTimeout = defaultProbeTimeout

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

// validateDim degrada a noop si el embedder no produce vectores de la dimensión
// del esquema: escribir otra dimensión rompe el INSERT contra la columna
// vector(N). El mismatch se detecta al boot (observable) y no en runtime.
//
// DOMAINSERV-80 H2: mide la dimensión REAL con un embed de prueba en vez de
// confiar en Dimensions(). Los 3 providers devolvían la constante del esquema
// sin mirar el modelo configurado, así que la comparación era siempre 1536 ==
// 1536 y el guard no protegía de nada: prender voyage (voyage-3 produce 1024)
// u ollama (nomic-embed-text produce 768) pasaba el guard y reventaba en cada
// escritura. Una tabla modelo→dimensión tendría el mismo defecto de fondo —
// quedaría desactualizada ante cada modelo nuevo. Medir no se desactualiza.
func validateDim(e llm.Embedder, logger *slog.Logger) llm.Embedder {
	if _, isNop := e.(llm.NopEmbedder); isNop {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	v, err := e.Embed(ctx, "dimension probe")
	if err != nil {
		logger.Warn("embedder: no se pudo medir la dimensión real; degradando a noop",
			slog.String("error", err.Error()), slog.Int("declarada", e.Dimensions()))
		return llm.NopEmbedder{}
	}
	if len(v) != embeddingDim {
		logger.Warn("embedder: la dimensión producida no coincide con el esquema pgvector; degradando a noop",
			slog.Int("dim_real", len(v)), slog.Int("dim_declarada", e.Dimensions()),
			slog.Int("schema_dim", embeddingDim))
		return llm.NopEmbedder{}
	}
	return e
}
