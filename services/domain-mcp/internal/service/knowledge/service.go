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
	"github.com/pgvector/pgvector-go"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/rag/chunker"
	"nunezlagos/domain/internal/service/knowledge/knowledgedb"
	"nunezlagos/domain/internal/store/txctx"
)

var (
	ErrTitleRequired = errors.New("title required")
	ErrBodyRequired  = errors.New("body required")
	ErrNotFound      = errors.New("knowledge document not found")
)

type Document struct {
	ID             uuid.UUID
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

func (s *Service) q(ctx context.Context) *knowledgedb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return knowledgedb.New(tx)
	}
	return knowledgedb.New(s.Pool)
}

func toDocument(d knowledgedb.GetDocRow) Document {
	var srcURL string
	if d.SourceUrl != nil {
		srcURL = *d.SourceUrl
	}
	var meta map[string]any
	if d.Metadata != nil {
		_ = json.Unmarshal(d.Metadata, &meta)
	}
	return Document{
		ID:             d.ID,
		ProjectID:      d.ProjectID,
		CreatedBy:      d.CreatedBy,
		Title:          d.Title,
		Body:           d.Body,
		Source:         d.Source,
		SourceURL:      srcURL,
		Tags:           d.Tags,
		Metadata:       meta,
		HasAttachments: d.HasAttachments,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
}

func toDocumentFromList(d knowledgedb.ListDocsByProjectRow) Document {
	var srcURL string
	if d.SourceUrl != nil {
		srcURL = *d.SourceUrl
	}
	var meta map[string]any
	if d.Metadata != nil {
		_ = json.Unmarshal(d.Metadata, &meta)
	}
	return Document{
		ID:             d.ID,
		ProjectID:      d.ProjectID,
		CreatedBy:      d.CreatedBy,
		Title:          d.Title,
		Body:           d.Body,
		Source:         d.Source,
		SourceURL:      srcURL,
		Tags:           d.Tags,
		Metadata:       meta,
		HasAttachments: d.HasAttachments,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
}

func toDocumentFromInsert(d knowledgedb.InsertDocRow) Document {
	var srcURL string
	if d.SourceUrl != nil {
		srcURL = *d.SourceUrl
	}
	var meta map[string]any
	if d.Metadata != nil {
		_ = json.Unmarshal(d.Metadata, &meta)
	}
	return Document{
		ID:             d.ID,
		ProjectID:      d.ProjectID,
		CreatedBy:      d.CreatedBy,
		Title:          d.Title,
		Body:           d.Body,
		Source:         d.Source,
		SourceURL:      srcURL,
		Tags:           d.Tags,
		Metadata:       meta,
		HasAttachments: d.HasAttachments,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
}

func toChunk(c knowledgedb.InsertChunkRow) Chunk {
	return Chunk{
		ID:         c.ID,
		DocumentID: c.KnowledgeDocID,
		ChunkIndex: int(c.ChunkIndex),
		Content:    c.Content,
		CreatedAt:  c.CreatedAt,
	}
}

func toChunkFromGet(c knowledgedb.GetChunksRow) Chunk {
	return Chunk{
		ID:         c.ID,
		DocumentID: c.KnowledgeDocID,
		ChunkIndex: int(c.ChunkIndex),
		Content:    c.Content,
		CreatedAt:  c.CreatedAt,
	}
}

func toSearchResult(r knowledgedb.SearchHybridRow) SearchResult {
	return SearchResult{
		DocumentID: r.KnowledgeDocID,
		ChunkID:    r.ID,
		ChunkIndex: int(r.ChunkIndex),
		Title:      r.Title,
		Snippet:    r.Content,
		Score:      r.Score,
		ProjectID:  r.ProjectID,
		CreatedAt:  r.CreatedAt,
	}
}

func toSearchResultBM25(r knowledgedb.SearchBm25Row) SearchResult {
	return SearchResult{
		DocumentID: r.KnowledgeDocID,
		ChunkID:    r.ID,
		ChunkIndex: int(r.ChunkIndex),
		Title:      r.Title,
		Snippet:    r.Content,
		Score:      r.Score,
		ProjectID:  r.ProjectID,
		CreatedAt:  r.CreatedAt,
	}
}

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

	rawChunks := chunker.Chunk(in.Body, s.ChunkOptions)
	if len(rawChunks) == 0 {
		return nil, nil, ErrBodyRequired
	}
	embeds, err := s.Embedder.EmbedBatch(ctx, rawChunks)
	if err != nil {
		return nil, nil, fmt.Errorf("embed chunks: %w", err)
	}

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := knowledgedb.New(tx)

	var srcURL *string
	if in.SourceURL != "" {
		srcURL = &in.SourceURL
	}

	docRow, err := q.InsertDoc(ctx, knowledgedb.InsertDocParams{
		ProjectID: in.ProjectID,
		CreatedBy: in.CreatedBy,
		Title:     in.Title,
		Body:      in.Body,
		Source:    in.Source,
		SourceUrl: srcURL,
		Tags:      in.Tags,
		Metadata:  metaJSON,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("insert doc: %w", err)
	}

	chunks := make([]Chunk, 0, len(rawChunks))
	for i, content := range rawChunks {
		var emb *pgvector.Vector
		if i < len(embeds) && len(embeds[i]) > 0 {
			v := pgvector.NewVector(embeds[i])
			emb = &v
		}
		chRow, err := q.InsertChunk(ctx, knowledgedb.InsertChunkParams{
			KnowledgeDocID: docRow.ID,
			ChunkIndex:     int32(i),
			Content:        content,
			Embedding:      emb,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("insert chunk %d: %w", i, err)
		}
		chunks = append(chunks, toChunk(chRow))
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("commit: %w", err)
	}

	doc := toDocumentFromInsert(docRow)

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
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

func (s *Service) Get(ctx context.Context, id uuid.UUID) (*Document, []Chunk, error) {
	docRow, err := s.q(ctx).GetDoc(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get doc: %w", err)
	}

	chunkRows, err := s.q(ctx).GetChunks(ctx, id)
	if err != nil {
		return nil, nil, fmt.Errorf("get chunks: %w", err)
	}

	doc := toDocument(docRow)
	chunks := make([]Chunk, len(chunkRows))
	for i, c := range chunkRows {
		chunks[i] = toChunkFromGet(c)
	}
	return &doc, chunks, nil
}

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

	if useVector {
		qvec := pgvector.NewVector(vec)
		rows, err := s.q(ctx).SearchHybrid(ctx, knowledgedb.SearchHybridParams{
			ResultLimit: int32(limit),
			QueryText:   query,
			Candidates:  int32(candidates),
			QueryVec:    &qvec,
			RrfK:        int32(rrfK),
		})
		if err != nil {
			return nil, fmt.Errorf("hybrid search: %w", err)
		}
		out := make([]SearchResult, len(rows))
		for i, r := range rows {
			out[i] = toSearchResult(r)
		}
		return out, nil
	}

	rows, err := s.q(ctx).SearchBm25(ctx, knowledgedb.SearchBm25Params{
		QueryText:   query,
		ResultLimit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("hybrid search (bm25 fallback): %w", err)
	}
	out := make([]SearchResult, len(rows))
	for i, r := range rows {
		out[i] = toSearchResultBM25(r)
	}
	return out, nil
}

func (s *Service) SoftDelete(ctx context.Context, id, actorID uuid.UUID) error {
	n, err := s.q(ctx).SoftDeleteDoc(ctx, id)
	if err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorID:    &actorID,
			ActorType:  audit.ActorUser,
			Action:     "knowledge_doc.deleted",
			EntityType: "knowledge_doc",
			EntityID:   &id,
		})
	}
	return nil
}

func (s *Service) ListByProject(ctx context.Context, projectID uuid.UUID, limit int) ([]Document, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.q(ctx).ListDocsByProject(ctx, knowledgedb.ListDocsByProjectParams{
		ProjectID:   projectID,
		ResultLimit: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list: %w", err)
	}
	out := make([]Document, len(rows))
	for i, r := range rows {
		out[i] = toDocumentFromList(r)
	}
	return out, nil
}
