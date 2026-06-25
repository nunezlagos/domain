// Package projectmerge — issue-01.5 fusiona un proyecto source en target.
//
// Migra observations, skills, flows, crons, agents desde source.project_id
// hacia target.project_id en una sola transacción. Tabla project_merges
// (000023) registra el merge para audit.
//
// Source project queda soft-deleted (deleted_at) post-merge para preservar
// historial y permitir rollback manual si surge problema.
//
// Convención de naming en conflictos (skills/agents/flows/crons que usan
// slug+project_id UNIQUE): el item del target prevalece; los del source
// con mismo slug se sufijean con "-merged-<source_slug>".
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
	SkillsRenamed     []string  `json:"skills_renamed,omitempty"`
	FlowsMoved        int       `json:"flows_moved"`
	FlowsRenamed      []string  `json:"flows_renamed,omitempty"`
	AgentsMoved       int       `json:"agents_moved"`
	AgentsRenamed     []string  `json:"agents_renamed,omitempty"`
	CronsMoved        int       `json:"crons_moved"`
	CronsRenamed      []string  `json:"crons_renamed,omitempty"`
	StartedAt         time.Time `json:"started_at"`
	CompletedAt       time.Time `json:"completed_at"`
}

// Service ejecuta merges atómicamente.
type Service struct {
	Pool  *pgxpool.Pool
	Audit *audit.PGRecorder
}

// Merge fusiona source → target. Atómico via tx; rollback completo en error.
func (s *Service) Merge(ctx context.Context, sourceID, targetID, actorID uuid.UUID) (*MergeReport, error) {
	if sourceID == targetID {
		return nil, ErrSameProject
	}

	report := MergeReport{
		MergeID:   uuid.New(),
		SourceID:  sourceID,
		TargetID:  targetID,
		StartedAt: time.Now(),
	}

	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)




	var sourceDeleted *time.Time
	var sourceSlug string
	err = tx.QueryRow(ctx,
		`SELECT deleted_at, slug FROM projects WHERE id = $1`, sourceID,
	).Scan(&sourceDeleted, &sourceSlug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: source", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("lookup source: %w", err)
	}
	if sourceDeleted != nil {
		return nil, ErrAlreadyMerged
	}



	err = tx.QueryRow(ctx,
		`SELECT 1 FROM projects WHERE id = $1 AND deleted_at IS NULL`, targetID,
	).Scan(new(int))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: target", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("lookup target: %w", err)
	}




	tag, err := tx.Exec(ctx,
		`UPDATE knowledge_observations SET project_id = $1 WHERE project_id = $2 AND deleted_at IS NULL`,
		targetID, sourceID,
	)
	if err != nil {
		return nil, fmt.Errorf("move observations: %w", err)
	}
	report.ObservationsMoved = int(tag.RowsAffected())



	for _, t := range []struct {
		table   string
		moved   *int
		renamed *[]string
	}{
		{"skills", &report.SkillsMoved, &report.SkillsRenamed},
		{"flows", &report.FlowsMoved, &report.FlowsRenamed},
		{"agents", &report.AgentsMoved, &report.AgentsRenamed},
		{"crons", &report.CronsMoved, &report.CronsRenamed},
	} {
		moved, renamed, err := moveWithRename(ctx, tx, t.table, sourceID, targetID, sourceSlug)
		if err != nil {
			return nil, fmt.Errorf("move %s: %w", t.table, err)
		}
		*t.moved = moved
		*t.renamed = renamed
	}


	if _, err := tx.Exec(ctx,
		`UPDATE projects SET deleted_at = now(), updated_at = now() WHERE id = $1`,
		sourceID,
	); err != nil {
		return nil, fmt.Errorf("soft-delete source: %w", err)
	}


	report.CompletedAt = time.Now()
	reportJSON, _ := json.Marshal(report)
	if _, err := tx.Exec(ctx,
		`INSERT INTO project_merges (id, source_id, target_id, actor_id, report, merged_at)
		 VALUES ($1, $2, $3, $4, $5, now())`,
		report.MergeID, sourceID, targetID, actorID, reportJSON,
	); err != nil {

		_, _ = tx.Exec(ctx,
			`INSERT INTO project_merges (id, source_id, target_id, merged_at)
			 VALUES ($1, $2, $3, now())`,
			report.MergeID, sourceID, targetID,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorType:  audit.ActorUser,
			ActorID:    &actorID,
			Action:     "project.merged",
			EntityType: "project",
			EntityID:   &targetID,
			NewValues: map[string]any{
				"merge_id":  report.MergeID.String(),
				"source_id": sourceID.String(),
				"counts": map[string]int{
					"observations": report.ObservationsMoved,
					"skills":       report.SkillsMoved,
					"flows":        report.FlowsMoved,
					"agents":       report.AgentsMoved,
					"crons":        report.CronsMoved,
				},
			},
		})
	}
	return &report, nil
}

// moveWithRename actualiza project_id en source → target. Para slugs que
// chocan con target, los sufijea con "-merged-<sourceSlug>" antes de mover.
func moveWithRename(ctx context.Context, tx pgx.Tx, table string, sourceID, targetID uuid.UUID, sourceSlug string) (int, []string, error) {

	rows, err := tx.Query(ctx,
		fmt.Sprintf(`
			SELECT s.id, s.slug
			FROM %s s
			WHERE s.project_id = $1
			  AND EXISTS (
			    SELECT 1 FROM %s t
			    WHERE t.project_id = $2 AND t.slug = s.slug
			  )`, table, table),
		sourceID, targetID,
	)
	if err != nil {
		return 0, nil, err
	}
	conflicts := []struct {
		ID   uuid.UUID
		Slug string
	}{}
	for rows.Next() {
		var c struct {
			ID   uuid.UUID
			Slug string
		}
		if err := rows.Scan(&c.ID, &c.Slug); err != nil {
			rows.Close()
			return 0, nil, err
		}
		conflicts = append(conflicts, c)
	}
	rows.Close()

	renamed := make([]string, 0, len(conflicts))
	for _, c := range conflicts {
		newSlug := c.Slug + "-merged-" + sourceSlug
		if _, err := tx.Exec(ctx,
			fmt.Sprintf(`UPDATE %s SET slug = $1 WHERE id = $2`, table),
			newSlug, c.ID,
		); err != nil {
			return 0, nil, fmt.Errorf("rename %s.%s: %w", table, c.Slug, err)
		}
		renamed = append(renamed, c.Slug+" → "+newSlug)
	}


	tag, err := tx.Exec(ctx,
		fmt.Sprintf(`UPDATE %s SET project_id = $1 WHERE project_id = $2`, table),
		targetID, sourceID,
	)
	if err != nil {
		return 0, renamed, err
	}
	return int(tag.RowsAffected()), renamed, nil
}
