package capturedprompt

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/capturedprompt/capturedpromptdb"
	"nunezlagos/domain/internal/store/txctx"
)

type pgRepository struct {
	pool *pgxpool.Pool
}

func NewPgRepository(pool *pgxpool.Pool) Repository {
	return &pgRepository{pool: pool}
}

func (r *pgRepository) q(ctx context.Context) *capturedpromptdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return capturedpromptdb.New(tx)
	}
	return capturedpromptdb.New(r.pool)
}

func toPrompt(id uuid.UUID, userID uuid.UUID, projectID *uuid.UUID, content, clientKind, model string, charCount, responseChars, estimatedTokensIn, estimatedTokensOut int32, capturedAt time.Time, turnCompletedAt pgtype.Timestamptz) Prompt {
	var tc *time.Time
	if turnCompletedAt.Valid {
		tc = &turnCompletedAt.Time
	}
	return Prompt{
		ID: id, UserID: userID, ProjectID: projectID,
		Content: content, ClientKind: clientKind, Model: model,
		CharCount: int(charCount), ResponseChars: int(responseChars),
		EstimatedTokensIn: int(estimatedTokensIn), EstimatedTokensOut: int(estimatedTokensOut),
		CapturedAt: capturedAt, TurnCompletedAt: tc,
	}
}

func toPromptFromInsert(r capturedpromptdb.InsertPromptRow) Prompt {
	return toPrompt(r.ID, r.UserID, r.ProjectID, r.Content, r.ClientKind, r.Model, r.CharCount, r.ResponseChars, r.EstimatedTokensIn, r.EstimatedTokensOut, r.CapturedAt, r.TurnCompletedAt)
}

func toPromptFromComplete(r capturedpromptdb.CompleteTurnRow) Prompt {
	return toPrompt(r.ID, r.UserID, r.ProjectID, r.Content, r.ClientKind, r.Model, r.CharCount, r.ResponseChars, r.EstimatedTokensIn, r.EstimatedTokensOut, r.CapturedAt, r.TurnCompletedAt)
}

func toPromptFromGet(r capturedpromptdb.GetPromptRow) Prompt {
	return toPrompt(r.ID, r.UserID, r.ProjectID, r.Content, r.ClientKind, r.Model, r.CharCount, r.ResponseChars, r.EstimatedTokensIn, r.EstimatedTokensOut, r.CapturedAt, r.TurnCompletedAt)
}

func toPromptFromList(r capturedpromptdb.ListPromptsRow) Prompt {
	return toPrompt(r.ID, r.UserID, r.ProjectID, r.Content, r.ClientKind, r.Model, r.CharCount, r.ResponseChars, r.EstimatedTokensIn, r.EstimatedTokensOut, r.CapturedAt, r.TurnCompletedAt)
}

// estimateTokens: ratio chars:tokens ≈ 4:1 (proxy estándar para
// español/inglés en modelos Anthropic/OpenAI). REQ-47.
func estimateTokens(chars int) int {
	if chars <= 0 {
		return 0
	}
	return (chars + 3) / 4 // ceil(chars/4)
}

func (r *pgRepository) Insert(ctx context.Context, in InsertParams) (*Prompt, error) {
	estIn := int32(estimateTokens(in.CharCount))

	row, err := r.q(ctx).InsertPrompt(ctx, capturedpromptdb.InsertPromptParams{
		UserID:            in.UserID,
		ProjectID:         in.ProjectID,
		Content:           in.Content,
		ClientKind:        in.ClientKind,
		Model:             in.Model,
		CharCount:         int32(in.CharCount),
		EstimatedTokensIn: estIn,
	})
	if err != nil {
		return nil, fmt.Errorf("insert captured_prompt: %w", err)
	}
	p := toPromptFromInsert(row)
	return &p, nil
}

func (r *pgRepository) CompleteTurn(ctx context.Context, in CompleteTurnInput) (*Prompt, error) {
	estOut := int32(estimateTokens(in.ResponseChars))

	row, err := r.q(ctx).CompleteTurn(ctx, capturedpromptdb.CompleteTurnParams{
		ID:                in.PromptID,
		ResponseChars:     int32(in.ResponseChars),
		EstimatedTokensOut: estOut,
		Model:             in.Model,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("complete turn: %w", err)
	}
	p := toPromptFromComplete(row)
	return &p, nil
}

func (r *pgRepository) SummarizeByProject(ctx context.Context, orgID, projectID uuid.UUID) (*SessionUsage, error) {
	_ = orgID // compat de firma

	row, err := r.q(ctx).SummarizeByProject(ctx, &projectID)
	if err != nil {
		return nil, fmt.Errorf("summarize by project: %w", err)
	}
	pid := projectID
	out := &SessionUsage{
		ProjectID:         &pid,
		Turns:             int(row.Turns),
		EstimatedTokensIn:  row.EstimatedTokensIn,
		EstimatedTokensOut: row.EstimatedTokensOut,
		TotalChars:         row.TotalChars,
	}
	return out, nil
}

func (r *pgRepository) Get(ctx context.Context, _ uuid.UUID, id uuid.UUID) (*Prompt, error) {
	row, err := r.q(ctx).GetPrompt(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get captured_prompt: %w", err)
	}
	p := toPromptFromGet(row)
	return &p, nil
}

func (r *pgRepository) List(ctx context.Context, _ uuid.UUID, filter ListFilter) ([]*Prompt, int64, error) {
	limit := int32(filter.Limit)
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	total, err := r.q(ctx).CountPrompts(ctx, capturedpromptdb.CountPromptsParams{
		ProjectID: filter.ProjectID,
		UserID:    filter.UserID,
	})
	if err != nil {
		return nil, 0, fmt.Errorf("count captured_prompts: %w", err)
	}

	rows, err := r.q(ctx).ListPrompts(ctx, capturedpromptdb.ListPromptsParams{
		ProjectID:    filter.ProjectID,
		UserID:       filter.UserID,
		ResultLimit:  limit,
		ResultOffset: int32(filter.Offset),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list captured_prompts: %w", err)
	}

	out := make([]*Prompt, 0, limit)
	for _, r := range rows {
		p := toPromptFromList(r)
		out = append(out, &p)
	}
	return out, total, nil
}
