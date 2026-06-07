// Package knowledge — HU-03.4 knowledge documents con chunking + RAG.
//
// Lifecycle:
//   - Save(title, body): persiste doc + chunkea + genera embedding por chunk
//   - Get(id): doc + chunks ordenados por chunk_index
//   - SearchSemantic(orgID, query, limit): cosine sobre chunks
//   - SearchHybrid(orgID, query, limit): BM25 chunks + cosine fused con RRF
//
// La generación de embeddings se delega a llm.Embedder inyectado. Si es
// NopEmbedder (vector zero), search degrada a tsvector-only.
package knowledge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/rag/chunker"
)

var (
	ErrTitleRequired = errors.New("title required")
	ErrBodyRequired  = errors.New("body required")
	ErrNotFound      = errors.New("knowledge document not found")
)

type Document struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	ProjectID      uuid.UUID
	CreatedBy      *uuid.UUID
	Title          string
	Body           string
	Source         string
	SourceURL      string
	Tags           []string
	Metadata       map[string]any
	HasAttachments bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Chunk struct {
	ID          uuid.UUID
	DocumentID  uuid.UUID
	ChunkIndex  int
	Content     string
	CreatedAt   time.Time
}

type SearchResult struct {
	DocumentID  uuid.UUID
	ChunkID     uuid.UUID
	ChunkIndex  int
	Title       string
	Snippet     string
	Score       float64
	ProjectID   uuid.UUID
	CreatedAt   time.Time
}

type SaveInput struct {
	OrganizationID uuid.UUID
	ProjectID      uuid.UUID
	CreatedBy      *uuid.UUID
	Title          string
	Body           string
	Source         string
	SourceURL      string
	Tags           []string
	Metadata       map[string]any
}

type Service struct {
	Pool         *pgxpool.Pool
	Audit        audit.Recorder
	Embedder     llm.Embedder
	ChunkOptions chunker.Options
}

// Save persiste el doc + sus chunks. Atómico: si falla un chunk, rollback todo.
// El embedding de cada chunk se genera ANTES de la tx (Embedder puede ser slow);
// si llm.Embedder.Embed falla, abortamos sin tocar DB.
func (s *Service) Save(ctx context.Context, in SaveInput) (*Document, []Chunk, error) {
	if strings.TrimSpace(in.Title) == "" {
		return nil, nil, ErrTitleRequired
	}
	if strings.TrimSpace(in.Body) == "" {
		return nil, nil, ErrBodyRequired
	}
	if in.Source == "" {
		in.Source = "manual"
	}
	if in.Tags == nil {
		in.Tags = []string{}
	}
	if in.Metadata == nil {
		in.Metadata = map[string]any{}
	}
	metaJSON, _ := json.Marshal(in.Metadata)

	// Chunk + embed ANTES de la tx para no mantener locks DB durante I/O LLM.
	rawChunks := chunker.Chunk(in.Body, s.ChunkOptions)
	if len(rawChunks) == 0 {
		return nil, nil, ErrBodyRequired
	}
	embeds := make([][]float32, len(rawChunks))
	for i, c := range rawChunks {
		v, err := s.Embedder.Embed(ctx, c)
		if err != nil {
			return nil, nil, fmt.Errorf("embed chunk %d: %w", i, err)
		}
		embeds[i] = v
	}

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var doc Document
	err = tx.QueryRow(ctx,
		`INSERT INTO knowledge_docs
		   (organization_id, project_id, created_by, title, body, source, source_url,
		    tags, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, organization_id, project_id, created_by, title, body,
		           source, COALESCE(source_url,''), tags, metadata,
		           has_attachments, created_at, updated_at`,
		in.OrganizationID, in.ProjectID, in.CreatedBy, in.Title, in.Body,
		in.Source, nullStr(in.SourceURL), in.Tags, metaJSON,
	).Scan(&doc.ID, &doc.OrganizationID, &doc.ProjectID, &doc.CreatedBy, &doc.Title, &doc.Body,
		&doc.Source, &doc.SourceURL, &doc.Tags, &doc.Metadata,
		&doc.HasAttachments, &doc.CreatedAt, &doc.UpdatedAt)
	if err != nil {
		return nil, nil, fmt.Errorf("insert doc: %w", err)
	}

	chunks := make([]Chunk, 0, len(rawChunks))
	for i, content := range rawChunks {
		var ch Chunk
		err := tx.QueryRow(ctx,
			`INSERT INTO knowledge_chunks (knowledge_doc_id, chunk_index, content, embedding)
			 VALUES ($1, $2, $3, $4::vector)
			 RETURNING id, knowledge_doc_id, chunk_index, content, created_at`,
			doc.ID, i, content, vectorLiteral(embeds[i]),
		).Scan(&ch.ID, &ch.DocumentID, &ch.ChunkIndex, &ch.Content, &ch.CreatedAt)
		if err != nil {
			return nil, nil, fmt.Errorf("insert chunk %d: %w", i, err)
		}
		chunks = append(chunks, ch)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit: %w", err)
	}

	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			OrganizationID: &in.OrganizationID,
			ActorID:        in.CreatedBy,
			ActorType:      audit.ActorUser,
			Action:         "knowledge_doc.saved",
			EntityType:     "knowledge_doc",
			EntityID:       &doc.ID,
			NewValues: map[string]any{
				"title": doc.Title, "chunks_count": len(chunks),
			},
		})
	}
	return &doc, chunks, nil
}

// Get devuelve doc + sus chunks ordenados por chunk_index.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Document, []Chunk, error) {
	var doc Document
	err := s.Pool.QueryRow(ctx,
		`SELECT id, organization_id, project_id, created_by, title, body,
		        source, COALESCE(source_url,''), tags, metadata,
		        has_attachments, created_at, updated_at
		 FROM knowledge_docs WHERE id = $1 AND deleted_at IS NULL`, id,
	).Scan(&doc.ID, &doc.OrganizationID, &doc.ProjectID, &doc.CreatedBy, &doc.Title, &doc.Body,
		&doc.Source, &doc.SourceURL, &doc.Tags, &doc.Metadata,
		&doc.HasAttachments, &doc.CreatedAt, &doc.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get doc: %w", err)
	}

	rows, err := s.Pool.Query(ctx,
		`SELECT id, knowledge_doc_id, chunk_index, content, created_at
		 FROM knowledge_chunks WHERE knowledge_doc_id = $1
		 ORDER BY chunk_index ASC`, id)
	if err != nil {
		return nil, nil, fmt.Errorf("get chunks: %w", err)
	}
	defer rows.Close()
	var chunks []Chunk
	for rows.Next() {
		var ch Chunk
		if err := rows.Scan(&ch.ID, &ch.DocumentID, &ch.ChunkIndex, &ch.Content, &ch.CreatedAt); err != nil {
			return nil, nil, err
		}
		chunks = append(chunks, ch)
	}
	return &doc, chunks, nil
}

// SearchHybrid combina BM25 sobre chunks + cosine RRF fusion.
// Si Embedder es Nop (zero), degrada a tsvector-only.
func (s *Service) SearchHybrid(ctx context.Context, orgID uuid.UUID, query string, limit int) ([]SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	vec, err := s.Embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	useVector := !llm.IsZero(vec)

	const rrfK = 60
	const candidates = 100

	var rows pgx.Rows
	if useVector {
		rows, err = s.Pool.Query(ctx, `
WITH bm25 AS (
  SELECT c.id, ROW_NUMBER() OVER (ORDER BY ts_rank(c.content_tsv, q) DESC) AS r
  FROM knowledge_chunks c
  JOIN knowledge_docs d ON d.id = c.knowledge_doc_id
       AND d.organization_id = $1 AND d.deleted_at IS NULL,
       plainto_tsquery('spanish', $2) AS q
  WHERE c.content_tsv @@ q
  LIMIT $4
),
vec AS (
  SELECT c.id, ROW_NUMBER() OVER (ORDER BY c.embedding <=> $3::vector ASC) AS r
  FROM knowledge_chunks c
  JOIN knowledge_docs d ON d.id = c.knowledge_doc_id
       AND d.organization_id = $1 AND d.deleted_at IS NULL
  WHERE c.embedding IS NOT NULL
  LIMIT $4
),
fused AS (
  SELECT id,
         COALESCE(1.0 / ($5 + bm25.r), 0) + COALESCE(1.0 / ($5 + vec.r), 0) AS score
  FROM bm25 FULL OUTER JOIN vec USING (id)
)
SELECT c.id, c.knowledge_doc_id, c.chunk_index, d.title, c.content,
       d.project_id, c.created_at, f.score
FROM fused f
JOIN knowledge_chunks c ON c.id = f.id
JOIN knowledge_docs d ON d.id = c.knowledge_doc_id
ORDER BY f.score DESC
LIMIT $6
`, orgID, query, vectorLiteral(vec), candidates, rrfK, limit)
	} else {
		rows, err = s.Pool.Query(ctx, `
SELECT c.id, c.knowledge_doc_id, c.chunk_index, d.title, c.content,
       d.project_id, c.created_at,
       ts_rank(c.content_tsv, q)::float8 AS score
FROM knowledge_chunks c
JOIN knowledge_docs d ON d.id = c.knowledge_doc_id
     AND d.organization_id = $1 AND d.deleted_at IS NULL,
     plainto_tsquery('spanish', $2) AS q
WHERE c.content_tsv @@ q
ORDER BY score DESC
LIMIT $3
`, orgID, query, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("hybrid search: %w", err)
	}
	defer rows.Close()
	var out []SearchResult
	for rows.Next() {
		var r SearchResult
		if err := rows.Scan(&r.ChunkID, &r.DocumentID, &r.ChunkIndex, &r.Title, &r.Snippet,
			&r.ProjectID, &r.CreatedAt, &r.Score); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SoftDelete marca el doc como deleted; los chunks viven CASCADE ON DELETE
// pero quedan accesibles para audit (no soft-delete en chunks individuales).
func (s *Service) SoftDelete(ctx context.Context, id, actorID uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx,
		`UPDATE knowledge_docs SET deleted_at = NOW()
		 WHERE id = $1 AND deleted_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
			ActorID:    &actorID,
			ActorType:  audit.ActorUser,
			Action:     "knowledge_doc.deleted",
			EntityType: "knowledge_doc",
			EntityID:   &id,
		})
	}
	return nil
}

// ListByProject docs activos del project, sin chunks (lite).
func (s *Service) ListByProject(ctx context.Context, projectID uuid.UUID, limit int) ([]Document, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx,
		`SELECT id, organization_id, project_id, created_by, title, body,
		        source, COALESCE(source_url,''), tags, metadata,
		        has_attachments, created_at, updated_at
		 FROM knowledge_docs
		 WHERE project_id = $1 AND deleted_at IS NULL
		 ORDER BY created_at DESC LIMIT $2`, projectID, limit)
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}
	defer rows.Close()
	var out []Document
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.OrganizationID, &d.ProjectID, &d.CreatedBy, &d.Title, &d.Body,
			&d.Source, &d.SourceURL, &d.Tags, &d.Metadata,
			&d.HasAttachments, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func vectorLiteral(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb, "%g", f)
	}
	sb.WriteByte(']')
	return sb.String()
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
