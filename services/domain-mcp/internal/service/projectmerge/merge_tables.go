package projectmerge

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"nunezlagos/domain/internal/service/projectmerge/projectmergedb"
)

// moveAll reasigna todas las entidades project-scoped de source a target y
// llena report con los conteos. El único conflicto posible es project_skills
// (UNIQUE(project_id,skill_id)): dedupe por skill_id. El resto ya no tiene
// unique per-project (000142 purgó los uniques org-based) → move directo.
func moveAll(ctx context.Context, tx pgx.Tx, db *projectmergedb.Queries, source, target uuid.UUID, r *MergeReport) error {
	n, err := db.MoveObservations(ctx, projectmergedb.MoveObservationsParams{TargetID: target, SourceID: source})
	if err != nil {
		return fmt.Errorf("move observations: %w", err)
	}
	r.ObservationsMoved = int(n)

	if r.SkillsMoved, r.SkillsDeduped, err = moveProjectSkills(ctx, tx, source, target); err != nil {
		return fmt.Errorf("move project_skills: %w", err)
	}
	for _, m := range []struct {
		table string
		dst   *int
	}{
		{"project_policies", &r.PoliciesMoved},
		{"prompts", &r.PromptsMoved},
		{"project_repositories", &r.ReposMoved},
		{"knowledge_docs", &r.DocsMoved},
		{"workflows", &r.WorkflowsMoved},
	} {
		if *m.dst, err = movePlain(ctx, tx, m.table, source, target); err != nil {
			return fmt.Errorf("move %s: %w", m.table, err)
		}
	}
	return nil
}

// movePlain reasigna project_id sin resolver conflictos (tablas sin unique
// per-project).
func movePlain(ctx context.Context, tx pgx.Tx, table string, source, target uuid.UUID) (int, error) {
	tag, err := tx.Exec(ctx, fmt.Sprintf(`UPDATE %s SET project_id = $1 WHERE project_id = $2`, table), target, source)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// moveProjectSkills dedupe por skill_id (UNIQUE(project_id,skill_id), sin slug):
// si el target ya tiene el skill, la fila del source se descarta antes de mover.
func moveProjectSkills(ctx context.Context, tx pgx.Tx, source, target uuid.UUID) (int, int, error) {
	del, err := tx.Exec(ctx, `
		DELETE FROM project_skills s
		 WHERE s.project_id = $1
		   AND EXISTS (SELECT 1 FROM project_skills t WHERE t.project_id = $2 AND t.skill_id = s.skill_id)`,
		source, target)
	if err != nil {
		return 0, 0, err
	}
	moved, err := movePlain(ctx, tx, "project_skills", source, target)
	if err != nil {
		return 0, 0, err
	}
	return moved, int(del.RowsAffected()), nil
}
