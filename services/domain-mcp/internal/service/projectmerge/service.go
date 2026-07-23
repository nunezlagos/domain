// Package projectmerge — DOMAINSERV-104: fusiona un proyecto source en target.
//
// Mueve las entidades project-scoped reales (knowledge_observations,
// project_skills, project_policies, project_repositories, knowledge_docs,
// prompts, workflows) de source a target en una sola tx serializable.
// flows/agents/crons son org-global (sin project_id) → NO se mueven; tickets e
// issues quedan fuera (namespace display_key per-project, DOMAINSERV-93 C).
//
// Tabla project_merges (000023) registra el merge para audit; el source queda
// soft-deleted post-merge. El único unique per-project que sobrevivió a la
// purga de organization_id (migración 000142) es project_skills(project_id,
// skill_id): se dedupe por skill_id. El resto de tablas ya no tienen unique de
// slug/name → move directo. No hay RLS activa sobre estas tablas.
package projectmerge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/service/projectmerge/projectmergedb"
)

var (
	ErrSameProject   = errors.New("source and target must be different")
	ErrCrossOrg      = errors.New("source and target must belong to same organization")
	ErrNotFound      = errors.New("project not found")
	ErrAlreadyMerged = errors.New("source project already merged")
)

// MergeReport documenta qué se movió.
type MergeReport struct {
	MergeID           uuid.UUID `json:"merge_id"`
	SourceID          uuid.UUID `json:"source_id"`
	TargetID          uuid.UUID `json:"target_id"`
	ObservationsMoved int       `json:"observations_moved"`
	SkillsMoved       int       `json:"skills_moved"`
	SkillsDeduped     int       `json:"skills_deduped,omitempty"`
	PoliciesMoved     int       `json:"policies_moved"`
	ReposMoved        int       `json:"repos_moved"`
	DocsMoved         int       `json:"docs_moved"`
	PromptsMoved      int       `json:"prompts_moved"`
	WorkflowsMoved    int       `json:"workflows_moved"`
	StartedAt         time.Time `json:"started_at"`
	CompletedAt       time.Time `json:"completed_at"`
}

// Service ejecuta merges atómicamente.
type Service struct {
	Pool  *pgxpool.Pool
	Audit *audit.PGRecorder
}

// Merge fusiona source → target. Atómico via tx serializable; rollback en error.
func (s *Service) Merge(ctx context.Context, sourceID, targetID, actorID uuid.UUID) (*MergeReport, error) {
	if sourceID == targetID {
		return nil, ErrSameProject
	}
	report := MergeReport{MergeID: uuid.New(), SourceID: sourceID, TargetID: targetID, StartedAt: time.Now()}

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	db := projectmergedb.New(tx)
	source, err := db.GetSourceProject(ctx, sourceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: source", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("lookup source: %w", err)
	}
	if source.DeletedAt.Valid {
		return nil, ErrAlreadyMerged
	}
	if _, err := db.CheckTargetExists(ctx, targetID); errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: target", ErrNotFound)
	} else if err != nil {
		return nil, fmt.Errorf("lookup target: %w", err)
	}

	if err := moveAll(ctx, tx, db, sourceID, targetID, &report); err != nil {
		return nil, err
	}
	if err := db.SoftDeleteProject(ctx, sourceID); err != nil {
		return nil, fmt.Errorf("soft-delete source: %w", err)
	}
	report.CompletedAt = time.Now()
	if err := insertMergeRecord(ctx, db, report, actorID); err != nil {
		return nil, fmt.Errorf("record merge: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	s.auditMerge(ctx, actorID, targetID, sourceID, report)
	return &report, nil
}

// insertMergeRecord persiste el trail en project_merges. actor_id es nullable
// (FK a users): uuid.Nil → NULL. En prod el actor es el user del Principal
// (siempre válido); un actor inválido no-nil aborta el merge (bug del caller).
func insertMergeRecord(ctx context.Context, db *projectmergedb.Queries, r MergeReport, actorID uuid.UUID) error {
	reportJSON, _ := json.Marshal(r)
	var actor *uuid.UUID
	if actorID != uuid.Nil {
		actor = &actorID
	}
	return db.InsertMergeRecord(ctx, projectmergedb.InsertMergeRecordParams{
		ID: r.MergeID, SourceID: r.SourceID, TargetID: r.TargetID, ActorID: actor, Report: reportJSON,
	})
}

func (s *Service) auditMerge(ctx context.Context, actorID, targetID, sourceID uuid.UUID, r MergeReport) {
	if s.Audit == nil {
		return
	}
	audit.RecordOrLog(ctx, s.Audit, audit.Event{
		ActorType: audit.ActorUser, ActorID: &actorID,
		Action: "project.merged", EntityType: "project", EntityID: &targetID,
		NewValues: map[string]any{
			"merge_id": r.MergeID.String(), "source_id": sourceID.String(),
			"counts": map[string]int{
				"observations": r.ObservationsMoved, "skills": r.SkillsMoved,
				"policies": r.PoliciesMoved, "repos": r.ReposMoved,
				"docs": r.DocsMoved, "prompts": r.PromptsMoved, "workflows": r.WorkflowsMoved,
			},
		},
	})
}
