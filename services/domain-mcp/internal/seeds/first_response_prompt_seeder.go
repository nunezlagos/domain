package seeds

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

const firstResponsePromptSlug = "first-response"

const DefaultFirstResponsePromptBody = `# First response after bootstrap - REGLA OBLIGATORIA

Siempre arranca asi:

## 1. Saludar + 1 linea contexto
"Buenas. En {proyecto} ({rama}), lo ultimo fue {tema ultima sesion}."

NO mas de 2 lineas para el saludo+contexto.

## 2. Llamar estos tools y compilar un bloque resumen
Despues del saludo, llama simultaneamente:
- ` + "`domain_project_skill_list(project_slug)`" + ` - skills del proyecto
- ` + "`domain_project_policy_list(project_slug)`" + ` - policies del proyecto
- ` + "`domain_ticket_list(project_slug, limit=5)`" + ` - tickets abiertos

Muestra el resultado en YAML:

` + "```" + `
slug:    {project_slug}
rama:    {branch}
remote:  {origin} [{kind}]
head:    {hash[:8]}

skills (project):  {N}
policies (project): {N}
tickets open:      {N}

ultimo:   {1 linea de la observation mas reciente}
` + "```" + `

Sin adornos, sin "veo que", sin explicar que tools llamaste.

## Caso tarea directa (no saludo)
Si el usuario dio una instruccion directa:
1. Mostrar el bloque YAML
2. Debajo, la respuesta a lo que pidio

## Prohibido siempre
- NO explicar que ejecutaste tools
- NO "segun el bootstrap"
- NO mas de ~12 lineas total en el bloque
- Si head.changed=true Y toca archivos criticos, agregar UNA linea de advertencia`

type FirstResponsePromptSeeder struct{}

func (s *FirstResponsePromptSeeder) Name() string    { return "first_response_prompt" }
func (s *FirstResponsePromptSeeder) Version() int    { return 1 }
func (s *FirstResponsePromptSeeder) Order() int      { return 63 }
func (s *FirstResponsePromptSeeder) IsDevOnly() bool { return false }

func (s *FirstResponsePromptSeeder) Run(ctx context.Context, tx pgx.Tx, _ Env) (Report, error) {
	var rep Report

	const description = "Prompt post-bootstrap: reglas de saludo + resumen YAML de proyecto/skills/policies/tickets en la primera respuesta. Editable desde el dashboard."
	body := strings.TrimSpace(DefaultFirstResponsePromptBody)

	var existingID string
	err := tx.QueryRow(ctx,
		`SELECT id::text FROM prompts
		 WHERE slug = $1 AND project_id IS NULL
		   AND is_active = true AND deleted_at IS NULL
		 ORDER BY version DESC LIMIT 1`,
		firstResponsePromptSlug,
	).Scan(&existingID)

	switch {
	case err == nil:
		if _, uerr := tx.Exec(ctx,
			`UPDATE prompts SET body = $1, description = $2 WHERE id = $3::uuid`,
			body, description, existingID,
		); uerr != nil {
			return rep, fmt.Errorf("update first-response prompt: %w", uerr)
		}
		rep.Updated++
	case errors.Is(err, pgx.ErrNoRows):
		if _, ierr := tx.Exec(ctx,
			`INSERT INTO prompts (project_id, created_by, slug, version,
			                      body, variables, description, is_active, tags)
			 VALUES (NULL, NULL, $1, 1, $2, '[]'::jsonb, $3, true, '{}')`,
			firstResponsePromptSlug, body, description,
		); ierr != nil {
			return rep, fmt.Errorf("insert first-response prompt: %w", ierr)
		}
		rep.Created++
	default:
		return rep, fmt.Errorf("query existing first-response prompt: %w", err)
	}

	return rep, nil
}
