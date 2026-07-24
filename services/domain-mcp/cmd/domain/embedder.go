package main

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

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

// defaultProbeTimeout acota CADA intento de medición: un provider lento o caído
// degrada a noop en vez de colgar el boot.
const defaultProbeTimeout = 10 * time.Second

// defaultProbeDeadline es el presupuesto TOTAL de la medición, sumando reintentos.
// Un intento único no alcanza: ollama carga el modelo a memoria durante el primer
// embed y eso tarda más que el timeout de un intento (bge-m3 en CPU, medido en el
// VPS: ~18s contra 10s). Ese desfasaje dejó la búsqueda semántica apagada tras un
// deploy que reportó éxito.
const defaultProbeDeadline = 90 * time.Second

// defaultProbePause espacia los reintentos. La carga sigue en curso del lado del
// provider aunque nuestro intento haya cortado por timeout, así que el siguiente
// suele encontrarla más avanzada.
const defaultProbePause = 5 * time.Second

// vars (no const) para que los tests no esperen los tiempos reales.
var (
	probeTimeout  = defaultProbeTimeout
	probeDeadline = defaultProbeDeadline
	probePause    = defaultProbePause
)

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

// warnIfSchemaDimDiffers compara embeddingDim contra la dimensión REAL de las
// columnas pgvector. Es el guard que faltaba: el de abajo verifica embedder vs
// constante, pero nadie verificaba constante vs esquema, y ese desalineo no se
// nota hasta que un INSERT falla en runtime — que fue exactamente lo que pasó al
// desplegar la migración 000275 con un binario que creía 1536.
//
// Solo avisa: el server tiene que arrancar igual para poder diagnosticar. Es la
// clase de error que se arregla redeployando, no reiniciando a ciegas.
func warnIfSchemaDimDiffers(ctx context.Context, pool interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, logger *slog.Logger) {
	if pool == nil {
		return
	}
	var dim int
	err := pool.QueryRow(ctx, `
		SELECT a.atttypmod FROM pg_attribute a
		JOIN pg_class c ON c.oid = a.attrelid
		JOIN pg_type t ON t.oid = a.atttypid
		WHERE t.typname = 'vector' AND c.relname = 'knowledge_observations'
		  AND a.attname = 'embedding'`).Scan(&dim)
	if err != nil || dim <= 0 {
		return
	}
	if dim != embeddingDim {
		logger.Error("DESALINEACIÓN: el esquema pgvector no coincide con el binario; "+
			"toda escritura de embedding va a fallar. Redeployá con el binario que corresponde a la migración aplicada",
			slog.Int("schema_dim", dim), slog.Int("binario_dim", embeddingDim))
	}
}

func buildEmbedder(logger *slog.Logger) llm.Embedder {
	kind := strings.ToLower(strings.TrimSpace(os.Getenv("DOMAIN_EMBEDDING_PROVIDER")))
	switch kind {
	case "", "noop":
		logger.Info("embedder: noop (búsqueda semántica desactivada; RAG cae a FTS)")
		return llm.NopEmbedder{Dim: embeddingDim}
	case "fake":
		logger.Warn("embedder: fake (determinístico, NO usar en prod)")
		return llm.FakeEmbedder{Dim: embeddingDim}
	case "openai":
		key := strings.TrimSpace(os.Getenv("DOMAIN_OPENAI_API_KEY"))
		if key == "" {
			logger.Warn("DOMAIN_EMBEDDING_PROVIDER=openai pero DOMAIN_OPENAI_API_KEY vacío; usando noop")
			return llm.NopEmbedder{Dim: embeddingDim}
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
			return llm.NopEmbedder{Dim: embeddingDim}
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
		return llm.NopEmbedder{Dim: embeddingDim}
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
	v, err := probeConReintentos(e, logger)
	if err != nil {
		logger.Warn("embedder: no se pudo medir la dimensión real; degradando a noop",
			slog.String("error", err.Error()), slog.Int("declarada", e.Dimensions()),
			slog.Duration("presupuesto", probeDeadline))
		return llm.NopEmbedder{Dim: embeddingDim}
	}
	if len(v) != embeddingDim {
		logger.Warn("embedder: la dimensión producida no coincide con el esquema pgvector; degradando a noop",
			slog.Int("dim_real", len(v)), slog.Int("dim_declarada", e.Dimensions()),
			slog.Int("schema_dim", embeddingDim))
		return llm.NopEmbedder{Dim: embeddingDim}
	}
	return e
}

// probeConReintentos insiste hasta agotar probeDeadline. Un fallo del embed es
// transitorio por definición durante el arranque —el provider puede estar
// levantando o cargando el modelo—, así que degradar al primer error confunde
// "todavía no" con "nunca". Una dimensión equivocada, en cambio, es definitiva:
// la devuelve el llamador sin reintentar.
func probeConReintentos(e llm.Embedder, logger *slog.Logger) ([]float32, error) {
	limite := time.Now().Add(probeDeadline)
	for intento := 1; ; intento++ {
		ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
		v, err := e.Embed(ctx, "dimension probe")
		cancel()
		if err == nil {
			return v, nil
		}
		if time.Now().Add(probePause).After(limite) {
			return nil, err
		}
		logger.Info("embedder: el provider todavía no responde el probe; reintentando",
			slog.Int("intento", intento), slog.String("error", err.Error()))
		time.Sleep(probePause)
	}
}
