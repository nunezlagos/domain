package seeds

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	analysissvc "nunezlagos/domain/internal/service/orchestrator/analysis"
)

// analysisPromptSlug es el slug del prompt del mini-pipeline de análisis
// read-only. El servicio (analysis.Service.PromptLoader) lo lee por este slug
// para usarlo como system prompt, de modo que editarlo en el dashboard cambia
// el análisis sin recompilar.
const analysisPromptSlug = "analysis"

// AnalysisPromptSeeder siembra el prompt de análisis (mini-pipeline read-only)
// en la tabla prompts con slug='analysis', project_id NULL (global), versión
// activa. Body = analysissvc.DefaultAnalysisSystemPrompt.
//
// Order 61: después del prompt de triage (60), mismo patrón.
//
// IDEMPOTENCIA: igual que TriagePromptSeeder — la tabla prompts NO tiene
// constraint único, así que se hace check-then-insert dentro de la tx (NO
// ON CONFLICT). El framework de seeders gatea por seed_versions, así que este
// Run solo corre cuando se bumpea Version().
type AnalysisPromptSeeder struct{}

func (s *AnalysisPromptSeeder) Name() string    { return "analysis_prompt" }
func (s *AnalysisPromptSeeder) Version() int    { return 2 } // 2: guard is_user_modified (DOMAINSERV-27)
func (s *AnalysisPromptSeeder) Order() int      { return 61 }
func (s *AnalysisPromptSeeder) IsDevOnly() bool { return false }

func (s *AnalysisPromptSeeder) Run(ctx context.Context, tx pgx.Tx, _ Env) (Report, error) {
	var rep Report

	const description = "System prompt del mini-pipeline de análisis read-only (intent analysis). Genera markdown estructurado. Editable desde el dashboard."
	body := strings.TrimSpace(analysissvc.DefaultAnalysisSystemPrompt)


	var existingID string
	err := tx.QueryRow(ctx,
		`SELECT id::text FROM prompts
		 WHERE slug = $1 AND project_id IS NULL
		   AND is_active = true AND deleted_at IS NULL
		 ORDER BY version DESC LIMIT 1`,
		analysisPromptSlug,
	).Scan(&existingID)

	switch {
	case err == nil:

		ct, uerr := tx.Exec(ctx,
			`UPDATE prompts SET body = $1, description = $2
			 WHERE id = $3::uuid AND NOT is_user_modified`,
			body, description, existingID,
		)
		if uerr != nil {
			return rep, fmt.Errorf("update analysis prompt: %w", uerr)
		}
		if ct.RowsAffected() == 0 {
			rep.Preserved++
		} else {
			rep.Updated++
		}
	case errors.Is(err, pgx.ErrNoRows):

		if _, ierr := tx.Exec(ctx,
			`INSERT INTO prompts (project_id, created_by, slug, version,
			                      body, variables, description, is_active, tags)
			 VALUES (NULL, NULL, $1, 1, $2, '[]'::jsonb, $3, true, '{}')`,
			analysisPromptSlug, body, description,
		); ierr != nil {
			return rep, fmt.Errorf("insert analysis prompt: %w", ierr)
		}
		rep.Created++
	default:
		return rep, fmt.Errorf("query existing analysis prompt: %w", err)
	}

	return rep, nil
}
