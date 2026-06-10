package seeds

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// ModelRegistrySeeder siembra el catálogo inicial de LLM models con pricing
// USD por 1M tokens. issue-01.7 + issue-06.4.
type ModelRegistrySeeder struct{}

func (s *ModelRegistrySeeder) Name() string    { return "model_registry" }
func (s *ModelRegistrySeeder) Version() int    { return 1 }
func (s *ModelRegistrySeeder) Order() int      { return 20 }
func (s *ModelRegistrySeeder) IsDevOnly() bool { return false }

type modelEntry struct {
	Provider, Model, Display    string
	Modality                    string
	Context                     int
	InputPer1M, OutputPer1M     float64
	EmbeddingDimensions         int
	Notes                       string
}

func (s *ModelRegistrySeeder) Run(ctx context.Context, tx pgx.Tx, env Env) (Report, error) {
	var rep Report

	models := []modelEntry{
		// Anthropic — Claude 4.x (latest family per CLAUDE.md context)
		{"anthropic", "claude-opus-4-7", "Claude Opus 4.7", "completion", 1_000_000, 15.0, 75.0, 0, "Most capable; complex reasoning, agentic tasks, 1M context window"},
		{"anthropic", "claude-sonnet-4-6", "Claude Sonnet 4.6", "completion", 200_000, 3.0, 15.0, 0, "Balanced; default for most agent workloads"},
		{"anthropic", "claude-haiku-4-5-20251001", "Claude Haiku 4.5", "completion", 200_000, 0.8, 4.0, 0, "Fast + cheap; intake classify, dedup, small tasks"},

		// OpenAI — GPT-4 family
		{"openai", "gpt-4o", "GPT-4o", "completion", 128_000, 2.5, 10.0, 0, "OpenAI flagship multimodal"},
		{"openai", "gpt-4o-mini", "GPT-4o mini", "completion", 128_000, 0.15, 0.6, 0, "Cheap GPT-4 class"},
		{"openai", "gpt-5", "GPT-5", "completion", 200_000, 10.0, 30.0, 0, "Newest GPT; use for complex agentic"},
		{"openai", "text-embedding-3-small", "Embedding 3 Small", "embedding", 8192, 0.02, 0, 1536, "Default embedder cheap"},
		{"openai", "text-embedding-3-large", "Embedding 3 Large", "embedding", 8192, 0.13, 0, 3072, "Higher quality embeddings"},

		// Voyage — embeddings especializados
		{"voyage", "voyage-3", "Voyage 3", "embedding", 32000, 0.06, 0, 1024, "Best-in-class retrieval embeddings"},
		{"voyage", "voyage-code-3", "Voyage Code 3", "embedding", 32000, 0.18, 0, 1024, "Code-optimized embeddings"},

		// Ollama — local models (sin pricing real, marcador)
		{"ollama", "llama3.3:70b", "Llama 3.3 70B (local)", "completion", 128_000, 0, 0, 0, "Local self-hosted; sin costo API"},
		{"ollama", "llama3.2:3b", "Llama 3.2 3B (local)", "completion", 128_000, 0, 0, 0, "Tiny local for dev / classify-only"},
		{"ollama", "nomic-embed-text", "Nomic Embed Text (local)", "embedding", 8192, 0, 0, 768, "Local embeddings sin costo"},

		// Google Gemini (referencia)
		{"google", "gemini-1.5-pro", "Gemini 1.5 Pro", "completion", 2_000_000, 1.25, 5.0, 0, "Largest context 2M tokens"},
		{"google", "gemini-1.5-flash", "Gemini 1.5 Flash", "completion", 1_000_000, 0.075, 0.3, 0, "Cheap fast Gemini"},
	}

	for _, m := range models {
		var embDim *int
		if m.EmbeddingDimensions > 0 {
			d := m.EmbeddingDimensions
			embDim = &d
		}
		var inP, outP *float64
		if m.InputPer1M > 0 {
			v := m.InputPer1M
			inP = &v
		}
		if m.OutputPer1M > 0 {
			v := m.OutputPer1M
			outP = &v
		}
		ctxSize := m.Context
		var ctxSizePtr *int
		if ctxSize > 0 {
			ctxSizePtr = &ctxSize
		}

		tag, err := tx.Exec(ctx, `
			INSERT INTO model_registry
			  (provider, model, display_name, modality, context_size,
			   input_per_million, output_per_million, embedding_dimensions, notes)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (provider, model) DO UPDATE
			SET display_name        = EXCLUDED.display_name,
			    modality            = EXCLUDED.modality,
			    context_size        = EXCLUDED.context_size,
			    input_per_million   = EXCLUDED.input_per_million,
			    output_per_million  = EXCLUDED.output_per_million,
			    embedding_dimensions = EXCLUDED.embedding_dimensions,
			    notes               = EXCLUDED.notes`,
			m.Provider, m.Model, m.Display, m.Modality, ctxSizePtr,
			inP, outP, embDim, m.Notes)
		if err != nil {
			return rep, fmt.Errorf("seed model %s/%s: %w", m.Provider, m.Model, err)
		}
		if tag.RowsAffected() == 1 {
			rep.Created++
		} else {
			rep.Updated++
		}
	}
	return rep, nil
}
