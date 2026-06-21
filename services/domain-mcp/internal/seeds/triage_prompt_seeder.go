package seeds

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"nunezlagos/domain/internal/service/promptrouter"
)

// triagePromptSlug es el slug del prompt de clasificación de intent. El
// classifier (promptrouter.LLMClassifier.PromptLoader) lo lee por este slug
// para usarlo como system prompt, de modo que editarlo en el dashboard
// cambia la clasificación sin recompilar.
const triagePromptSlug = "triage"

// TriagePromptSeeder siembra el prompt de triage (clasificación de intent)
// en la tabla prompts con slug='triage', project_id NULL (global), versión
// activa. Body = promptrouter.DefaultTriageSystemPrompt.
//
// Order 60: después de los catálogos (skills=50, agent_templates=51,
// flows=52) — no depende de ellos pero conviene seedearlo al final.
//
// IDEMPOTENCIA: la tabla prompts NO tiene constraint único (migración 000142
// dropeó UNIQUE(organization_id, project_id, slug, version) junto con la
// columna organization_id, y NO se recreó — la unicidad de "activa por slug"
// se enforcea en el service, no en DB). Por eso NO se puede usar ON CONFLICT.
// El seeder hace check-then-insert dentro de la tx: si ya hay una versión
// activa del slug con project_id NULL, actualiza su body; si no, inserta
// version 1 activa. El framework de seeders además gatea por seed_versions,
// así que este Run solo corre cuando se bumpea Version().
type TriagePromptSeeder struct{}

func (s *TriagePromptSeeder) Name() string    { return "triage_prompt" }
func (s *TriagePromptSeeder) Version() int    { return 1 }
func (s *TriagePromptSeeder) Order() int      { return 60 }
func (s *TriagePromptSeeder) IsDevOnly() bool { return false }

func (s *TriagePromptSeeder) Run(ctx context.Context, tx pgx.Tx, _ Env) (Report, error) {
	var rep Report

	const description = "Clasificador de intent del prompt router (chat/idea/feature/fix/hotfix/refactor/doc/rfc/analysis). Editable desde el dashboard."
	body := strings.TrimSpace(promptrouter.DefaultTriageSystemPrompt)

	// ¿Ya existe una versión activa global (project_id NULL) del slug?
	var existingID string
	err := tx.QueryRow(ctx,
		`SELECT id::text FROM prompts
		 WHERE slug = $1 AND project_id IS NULL
		   AND is_active = true AND deleted_at IS NULL
		 ORDER BY version DESC LIMIT 1`,
		triagePromptSlug,
	).Scan(&existingID)

	switch {
	case err == nil:
		// Existe: actualizar body + description (refresh del catálogo).
		if _, uerr := tx.Exec(ctx,
			`UPDATE prompts SET body = $1, description = $2 WHERE id = $3::uuid`,
			body, description, existingID,
		); uerr != nil {
			return rep, fmt.Errorf("update triage prompt: %w", uerr)
		}
		rep.Updated++
	case errors.Is(err, pgx.ErrNoRows):
		// No existe: insertar version 1 activa, global (project_id NULL).
		if _, ierr := tx.Exec(ctx,
			`INSERT INTO prompts (project_id, created_by, slug, version,
			                      body, variables, description, is_active, tags)
			 VALUES (NULL, NULL, $1, 1, $2, '[]'::jsonb, $3, true, '{}')`,
			triagePromptSlug, body, description,
		); ierr != nil {
			return rep, fmt.Errorf("insert triage prompt: %w", ierr)
		}
		rep.Created++
	default:
		return rep, fmt.Errorf("query existing triage prompt: %w", err)
	}

	return rep, nil
}
