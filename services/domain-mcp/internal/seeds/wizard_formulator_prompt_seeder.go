package seeds

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"nunezlagos/domain/internal/service/wizardplan"
)

// wizardFormulatorPromptSlug es el slug del prompt del wizard formulator. El
// formulator (wizardplan.LLMQuestionFormulator.PromptLoader) lo lee por este
// slug para usarlo como system prompt (skeleton), de modo que editarlo en el
// dashboard cambia la formulación de preguntas sin recompilar. El envelope
// dinámico interpolado en el mensaje user NO depende de este slug.
const wizardFormulatorPromptSlug = "wizard-formulator"

// WizardFormulatorPromptSeeder siembra el prompt del wizard formulator en la
// tabla prompts con slug='wizard-formulator', project_id NULL (global),
// versión activa. Body = wizardplan.DefaultFormulatorSystemPrompt.
//
// Order 62: después de triage (60) y analysis (61), mismo patrón.
//
// IDEMPOTENCIA: igual que TriagePromptSeeder — la tabla prompts NO tiene
// constraint único, así que se hace check-then-insert dentro de la tx (NO
// ON CONFLICT). El framework de seeders gatea por seed_versions, así que este
// Run solo corre cuando se bumpea Version().
type WizardFormulatorPromptSeeder struct{}

func (s *WizardFormulatorPromptSeeder) Name() string    { return "wizard_formulator_prompt" }
func (s *WizardFormulatorPromptSeeder) Version() int    { return 1 }
func (s *WizardFormulatorPromptSeeder) Order() int      { return 62 }
func (s *WizardFormulatorPromptSeeder) IsDevOnly() bool { return false }

func (s *WizardFormulatorPromptSeeder) Run(ctx context.Context, tx pgx.Tx, _ Env) (Report, error) {
	var rep Report

	const description = "System prompt del wizard formulator (formula preguntas naturales para clarificar slots de una HU). Skeleton editable desde el dashboard; el envelope dinámico se interpola aparte."
	body := strings.TrimSpace(wizardplan.DefaultFormulatorSystemPrompt)

	// ¿Ya existe una versión activa global (project_id NULL) del slug?
	var existingID string
	err := tx.QueryRow(ctx,
		`SELECT id::text FROM prompts
		 WHERE slug = $1 AND project_id IS NULL
		   AND is_active = true AND deleted_at IS NULL
		 ORDER BY version DESC LIMIT 1`,
		wizardFormulatorPromptSlug,
	).Scan(&existingID)

	switch {
	case err == nil:
		// Existe: actualizar body + description (refresh del catálogo).
		if _, uerr := tx.Exec(ctx,
			`UPDATE prompts SET body = $1, description = $2 WHERE id = $3::uuid`,
			body, description, existingID,
		); uerr != nil {
			return rep, fmt.Errorf("update wizard-formulator prompt: %w", uerr)
		}
		rep.Updated++
	case errors.Is(err, pgx.ErrNoRows):
		// No existe: insertar version 1 activa, global (project_id NULL).
		if _, ierr := tx.Exec(ctx,
			`INSERT INTO prompts (project_id, created_by, slug, version,
			                      body, variables, description, is_active, tags)
			 VALUES (NULL, NULL, $1, 1, $2, '[]'::jsonb, $3, true, '{}')`,
			wizardFormulatorPromptSlug, body, description,
		); ierr != nil {
			return rep, fmt.Errorf("insert wizard-formulator prompt: %w", ierr)
		}
		rep.Created++
	default:
		return rep, fmt.Errorf("query existing wizard-formulator prompt: %w", err)
	}

	return rep, nil
}
