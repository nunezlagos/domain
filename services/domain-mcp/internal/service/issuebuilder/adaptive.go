package issuebuilder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"

	wp "nunezlagos/domain/internal/service/wizardplan"
)

// AdaptiveService es la API v2 del wizard (issue-04.7 reemplaza v1 de 8
// preguntas fijas). El cliente solo necesita StartAdaptive + AnswerAdaptive.
//
// Persiste el envelope en issue_drafts.answers["__envelope__"] y los slots
// provided en issue_drafts.answers[slot_key].
type AdaptiveService struct {
	*Service
	Analyzer *wp.Analyzer
	Planner  *wp.Planner
}

// StartAdaptive corre el análisis pipeline sobre el prompt + crea draft
// + devuelve la primera pregunta (o nil si todo se infirió).
func (a *AdaptiveService) StartAdaptive(ctx context.Context, rawPrompt string, createdBy *uuid.UUID, projectID *uuid.UUID) (*Draft, *wp.Question, error) {
	env, err := a.Analyzer.Analyze(ctx, rawPrompt)
	if err != nil {
		return nil, nil, fmt.Errorf("analyze: %w", err)
	}

	intent := "feature"
	if env.Intent != nil {
		intent = env.Intent.Intent
	}
	mode := modeFromIntent(intent)
	if mode == "" {

		return nil, nil, fmt.Errorf("intent=%s no requiere wizard", intent)
	}


	d, _, err := a.Service.Start(ctx, mode, rawPrompt, createdBy, projectID)
	if err != nil {
		return nil, nil, err
	}

	if err := a.persistEnvelope(ctx, d.ID, env); err != nil {
		return d, nil, fmt.Errorf("persist envelope: %w", err)
	}

	q, qErr := a.Planner.NextQuestion(ctx, env)
	if errors.Is(qErr, wp.NoMoreQuestionsErr) {

		_, _ = a.Pool.Exec(ctx,
			`UPDATE issue_drafts SET status = $1, current_step = total_steps, updated_at = now() WHERE id = $2`,
			StatusFinished, d.ID,
		)
		d.Status = StatusFinished
		return d, nil, nil
	}
	if qErr != nil {
		return d, nil, qErr
	}
	return d, q, nil
}

// AnswerAdaptive recibe la respuesta del usuario para el slot actual,
// la persiste en el envelope, y devuelve la próxima pregunta o nil si
// ya está listo el preview.
func (a *AdaptiveService) AnswerAdaptive(ctx context.Context, draftID uuid.UUID, slotKey string, value any) (*Draft, *wp.Question, error) {
	env, err := a.loadEnvelope(ctx, draftID)
	if err != nil {
		return nil, nil, err
	}
	d, err := a.Get(ctx, draftID)
	if err != nil {
		return nil, nil, err
	}
	if d.Status != StatusInProgress {
		return nil, nil, fmt.Errorf("%w: status=%s", ErrInvalidStatus, d.Status)
	}

	a.Planner.RecordAnswer(env, slotKey, value)

	if err := a.persistEnvelope(ctx, draftID, env); err != nil {
		return nil, nil, fmt.Errorf("persist envelope: %w", err)
	}

	q, qErr := a.Planner.NextQuestion(ctx, env)
	if errors.Is(qErr, wp.NoMoreQuestionsErr) {
		_, _ = a.Pool.Exec(ctx,
			`UPDATE issue_drafts SET status = $1, current_step = total_steps, updated_at = now() WHERE id = $2`,
			StatusFinished, draftID,
		)
		d.Status = StatusFinished
		return d, nil, nil
	}
	if qErr != nil {
		return d, nil, qErr
	}
	return d, q, nil
}

// LoadEnvelope expone el envelope persistido para que el cliente lo lea
// (útil para mostrar al usuario QUÉ se infirió antes de cada pregunta).
func (a *AdaptiveService) LoadEnvelope(ctx context.Context, draftID uuid.UUID) (*wp.ContextEnvelope, error) {
	return a.loadEnvelope(ctx, draftID)
}

func (a *AdaptiveService) persistEnvelope(ctx context.Context, draftID uuid.UUID, env *wp.ContextEnvelope) error {
	answers := map[string]any{}
	row := a.Pool.QueryRow(ctx,
		`SELECT COALESCE(answers, '{}'::jsonb) FROM issue_drafts WHERE id = $1`, draftID)
	var raw json.RawMessage
	if err := row.Scan(&raw); err != nil {
		return err
	}
	_ = json.Unmarshal(raw, &answers)

	answers["__envelope__"] = env



	for k, slot := range env.Slots {
		if slot.Status == wp.SlotProvided || slot.Status == wp.SlotConfirmed {
			answers[k] = slot.Value
		}
	}

	body, _ := json.Marshal(answers)
	_, err := a.Pool.Exec(ctx,
		`UPDATE issue_drafts SET answers = $1, updated_at = now() WHERE id = $2`,
		body, draftID,
	)
	return err
}

func (a *AdaptiveService) loadEnvelope(ctx context.Context, draftID uuid.UUID) (*wp.ContextEnvelope, error) {
	var raw json.RawMessage
	err := a.Pool.QueryRow(ctx,
		`SELECT COALESCE(answers, '{}'::jsonb) FROM issue_drafts WHERE id = $1`, draftID,
	).Scan(&raw)
	if err != nil {
		return nil, fmt.Errorf("load envelope: %w", err)
	}
	var answers map[string]json.RawMessage
	_ = json.Unmarshal(raw, &answers)
	if envRaw, ok := answers["__envelope__"]; ok {
		env, err := wp.UnmarshalFromJSON(envRaw)
		if err == nil {
			return env, nil
		}
	}
	return wp.NewEnvelope("", "feature"), nil
}

func modeFromIntent(intent string) string {
	switch intent {
	case "fix", "hotfix":
		return ModeBugFix
	case "refactor":
		return ModeRefactor
	case "doc":
		return ModeDoc
	case "rfc":
		return ModeRFC
	case "feature":
		return ModeFeature
	}
	return ""
}
