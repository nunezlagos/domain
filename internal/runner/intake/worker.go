// Package intake — issue-04.8 worker async para el intake pipeline.
//
// Polea status='received' → ejecuta classify (LLM call) → dedup (semantic
// search) → structure (LLM call) → marca pending_review. Si LLM/skill no
// disponible, queda en status=failed con error registrado.
package intake

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/llm"
	"nunezlagos/domain/internal/service/intake"
)

// Classifier es la interfaz que un wrapper LLM debe satisfacer para classify.
type Classifier interface {
	Classify(ctx context.Context, rawText string) (Classification, error)
}

// Classification es el output esperado del classify step.
type Classification struct {
	Type       string  `json:"type"`         // feat|fix|hotfix|chore|refactor|docs
	Severity   string  `json:"severity"`     // low|medium|high|critical
	Confidence float64 `json:"confidence"`   // 0..1
	Reasoning  string  `json:"reasoning"`
}

// Structurer convierte raw text + classification en proposed title +
// description + req_slug + hu_draft.
type Structurer interface {
	Structure(ctx context.Context, rawText string, cls Classification) (Structured, error)
}

// Structured es el output del structure step.
type Structured struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	ReqSlug     string         `json:"req_slug"`
	IssueDraftWizard     map[string]any `json:"hu_draft"`
}

// DedupSearcher busca duplicados via embedding del proposed_title + description.
type DedupSearcher interface {
	FindCandidates(ctx context.Context, embedding []float32, threshold float64, limit int) ([]DedupCandidate, error)
}

// DedupCandidate refleja un match potencial.
type DedupCandidate struct {
	ReqID      *uuid.UUID `json:"req_id,omitempty"`
	HUID       *uuid.UUID `json:"issue_id,omitempty"`
	Title      string     `json:"title"`
	Similarity float64    `json:"similarity"`
	Reason     string     `json:"reason"`
}

// Worker process intake_payloads pendientes.
type Worker struct {
	Service     *intake.Service
	Classifier  Classifier
	Structurer  Structurer
	DedupSearch DedupSearcher
	Embedder    llm.Embedder
	Logger      *slog.Logger

	// PollInterval es el tiempo entre intentos. Default 30s.
	PollInterval time.Duration
	// BatchSize es cuántos intakes procesa por tick. Default 5.
	BatchSize int
	// DedupThreshold default 0.75.
	DedupThreshold float64
	// MergeThreshold (≥) sugiere append_to_hu en lugar de create_new. Default 0.92.
	MergeThreshold float64
}

// Run corre el worker en loop hasta context cancellation.
func (w *Worker) Run(ctx context.Context) error {
	if w.PollInterval <= 0 {
		w.PollInterval = 30 * time.Second
	}
	if w.BatchSize <= 0 {
		w.BatchSize = 5
	}
	if w.DedupThreshold <= 0 {
		w.DedupThreshold = 0.75
	}
	if w.MergeThreshold <= 0 {
		w.MergeThreshold = 0.92
	}
	if w.Logger == nil {
		w.Logger = slog.Default()
	}

	t := time.NewTicker(w.PollInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			w.tick(ctx)
		}
	}
}

func (w *Worker) tick(ctx context.Context) {
	pending, err := w.Service.ListPending(ctx, w.BatchSize)
	if err != nil {
		w.Logger.Warn("intake worker list failed", slog.Any("err", err))
		return
	}
	for i := range pending {
		p := pending[i]
		if p.Status != intake.StatusReceived {
			continue
		}
		if err := w.ProcessOne(ctx, &p); err != nil {
			w.Logger.Warn("intake processing failed",
				slog.String("intake_id", p.ID.String()),
				slog.Any("err", err))
		}
	}
}

// ProcessOne ejecuta classify → dedup → structure en un intake.
// Exportado para test directo.
func (w *Worker) ProcessOne(ctx context.Context, p *intake.Payload) error {
	if w.Classifier == nil || w.Structurer == nil {
		return errors.New("classifier and structurer are required")
	}

	// 1. Classify
	cls, err := w.Classifier.Classify(ctx, p.RawText)
	if err != nil {
		return fmt.Errorf("classify: %w", err)
	}
	if _, err := w.Service.UpdateClassification(ctx, p.ID,
		cls.Type, cls.Severity, cls.Confidence, cls.Reasoning); err != nil {
		return fmt.Errorf("persist classification: %w", err)
	}

	// 2. Structure (produce proposed title + description + hu_draft).
	st, err := w.Structurer.Structure(ctx, p.RawText, cls)
	if err != nil {
		return fmt.Errorf("structure: %w", err)
	}

	// 3. Dedup vs existing requirements/HUs.
	var candidates []DedupCandidate
	merge := intake.MergeActionCreateNew
	if w.Embedder != nil && w.DedupSearch != nil {
		query := st.Title + "\n" + st.Description
		emb, err := w.Embedder.Embed(ctx, query)
		if err == nil && len(emb) > 0 {
			candidates, _ = w.DedupSearch.FindCandidates(ctx, emb, w.DedupThreshold, 5)
			for _, c := range candidates {
				if c.Similarity >= w.MergeThreshold {
					merge = intake.MergeActionAppendToHU
					break
				}
			}
		}
	}
	dedupAny := make([]any, 0, len(candidates))
	for _, c := range candidates {
		dedupAny = append(dedupAny, c)
	}

	if _, err := w.Service.MarkPendingReview(ctx, p.ID,
		st.Title, st.Description, st.ReqSlug, st.IssueDraftWizard, dedupAny, merge); err != nil {
		return fmt.Errorf("mark pending review: %w", err)
	}

	w.Logger.Info("intake processed",
		slog.String("intake_id", p.ID.String()),
		slog.String("type", cls.Type),
		slog.String("severity", cls.Severity),
		slog.Float64("confidence", cls.Confidence),
		slog.Int("dedup_candidates", len(candidates)),
		slog.String("merge_action", merge))
	return nil
}

// StubClassifier devuelve clasificación canned (uso dev/test sin LLM).
type StubClassifier struct {
	DefaultType     string
	DefaultSeverity string
	DefaultConfidence float64
}

// Classify implements Classifier.
func (s *StubClassifier) Classify(_ context.Context, _ string) (Classification, error) {
	t := s.DefaultType
	if t == "" {
		t = "feat"
	}
	sev := s.DefaultSeverity
	if sev == "" {
		sev = "medium"
	}
	conf := s.DefaultConfidence
	if conf == 0 {
		conf = 0.5
	}
	return Classification{
		Type: t, Severity: sev, Confidence: conf,
		Reasoning: "stub classifier (no LLM)",
	}, nil
}

// StubStructurer extrae primeros 80 chars como title; resto como description.
type StubStructurer struct {
	DefaultReqSlug string
}

// Structure implements Structurer.
func (s *StubStructurer) Structure(_ context.Context, raw string, _ Classification) (Structured, error) {
	title := raw
	desc := raw
	if len(title) > 80 {
		title = title[:80] + "…"
	}
	req := s.DefaultReqSlug
	if req == "" {
		req = "REQ-XX-uncategorized"
	}
	return Structured{
		Title: title, Description: desc, ReqSlug: req,
		IssueDraftWizard: map[string]any{
			"slug":  "auto-" + json.Number(fmt.Sprintf("%d", time.Now().Unix())).String(),
			"goal":  "auto-generated stub",
		},
	}, nil
}
