// issue-07.3 llm-semantic-cache — cache de responses LLM por similaridad
// semántica de prompts (no exact match). Si el prompt nuevo tiene cosine
// similarity >= 0.95 con uno cacheado del mismo (provider, model, params),
// devolvemos el response cacheado y ahorramos la llamada al provider.
package semcache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Entry es una entrada cacheada.
type Entry struct {
	ID            string    `json:"id"`
	OrgID         string    `json:"organization_id"`
	Provider      string    `json:"provider"`
	Model         string    `json:"model"`
	ParamsHash    string    `json:"params_hash"`
	PromptHash    string    `json:"prompt_hash"`
	PromptPreview string    `json:"prompt_preview"`
	Response      json.RawMessage `json:"response"`
	Tokens        int       `json:"tokens"`
	HitCount      int       `json:"hit_count"`
	CreatedAt     time.Time `json:"created_at"`
	LastUsedAt    time.Time `json:"last_used_at"`
}

// Lookup result.
type Hit struct {
	Entry      *Entry  `json:"entry"`
	Similarity float64 `json:"similarity"`
}

// Cache gestiona persistencia + lookup vector.
type Cache struct {
	Pool          *pgxpool.Pool
	MinSimilarity float64       // default 0.95
	TTL           time.Duration // default 7 días
}

var ErrCacheMiss = errors.New("cache miss")

// HashParams calcula hash determinístico de params para distinguir cache
// entries del mismo prompt con distintos temperature/top_p/etc.
func HashParams(params map[string]any) string {
	canonical, _ := json.Marshal(params)
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

// HashPrompt sirve como cheap-key de comparación (para dedup exact).
func HashPrompt(prompt string) string {
	sum := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(sum[:])
}

// Lookup busca un entry con cosine similarity >= MinSimilarity sobre el
// embedding del prompt. Filtra por (org_id, provider, model, paramsHash)
// para que distintos contextos no se mezclen.
func (c *Cache) Lookup(ctx context.Context, orgID, provider, model, paramsHash string, embedding []float32) (*Hit, error) {
	minSim := c.MinSimilarity
	if minSim <= 0 {
		minSim = 0.95
	}
	ttl := c.TTL
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	cutoff := time.Now().Add(-ttl)

	var e Entry
	var sim float64
	vecLit := vectorLiteral(embedding)
	// ISSUE-21.6 Fase D clean: single-org. WHERE sin organization_id.
	err := c.Pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, organization_id, provider, model, params_hash, prompt_hash,
		       prompt_preview, response, tokens, hit_count, created_at, last_used_at,
		       1 - (prompt_embedding <=> %s) AS similarity
		FROM llm_semantic_cache
		WHERE provider = $1
		  AND model = $2
		  AND params_hash = $3
		  AND created_at >= $4
		  AND (1 - (prompt_embedding <=> %s)) >= $5
		ORDER BY prompt_embedding <=> %s
		LIMIT 1`, vecLit, vecLit, vecLit),
		orgID, provider, model, paramsHash, cutoff, minSim,
	).Scan(&e.ID, &e.OrgID, &e.Provider, &e.Model, &e.ParamsHash, &e.PromptHash,
		&e.PromptPreview, &e.Response, &e.Tokens, &e.HitCount,
		&e.CreatedAt, &e.LastUsedAt, &sim)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrCacheMiss
	}
	if err != nil {
		return nil, fmt.Errorf("lookup: %w", err)
	}

	// Bump hit_count + last_used_at (fire-and-forget; no necesita tx)
	_, _ = c.Pool.Exec(ctx,
		`UPDATE llm_semantic_cache SET hit_count = hit_count + 1, last_used_at = now() WHERE id = $1`,
		e.ID,
	)

	return &Hit{Entry: &e, Similarity: sim}, nil
}

// Store persiste un (prompt, response) en cache. Idempotente: ON CONFLICT
// (org, provider, model, params_hash, prompt_hash) actualiza response.
func (c *Cache) Store(ctx context.Context, e Entry, embedding []float32) error {
	vecLit := vectorLiteral(embedding)
	_, err := c.Pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO llm_semantic_cache
		  (id, organization_id, provider, model, params_hash, prompt_hash,
		   prompt_preview, response, tokens, prompt_embedding)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, %s)
		ON CONFLICT (organization_id, provider, model, params_hash, prompt_hash)
		DO UPDATE SET response = EXCLUDED.response, tokens = EXCLUDED.tokens,
		              last_used_at = now()`, vecLit),
		e.ID, e.OrgID, e.Provider, e.Model, e.ParamsHash, e.PromptHash,
		e.PromptPreview, e.Response, e.Tokens,
	)
	if err != nil {
		return fmt.Errorf("upsert: %w", err)
	}
	return nil
}

// vectorLiteral convierte []float32 a literal pgvector inline '[v1,v2,...]'::vector.
// Espejo simple del helper en internal/llm para evitar import cycle.
func vectorLiteral(v []float32) string {
	if len(v) == 0 {
		return "'[]'::vector"
	}
	var b strings.Builder
	b.WriteByte('\'')
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(fmt.Sprintf("%g", f))
	}
	b.WriteByte(']')
	b.WriteByte('\'')
	b.WriteString("::vector")
	return b.String()
}

// Evict vacía entries más viejas que TTL (cron cleanup).
func (c *Cache) Evict(ctx context.Context) (int, error) {
	ttl := c.TTL
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	tag, err := c.Pool.Exec(ctx,
		`DELETE FROM llm_semantic_cache WHERE last_used_at < $1`,
		time.Now().Add(-ttl),
	)
	if err != nil {
		return 0, fmt.Errorf("evict: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
