package seeds

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

const firstResponsePromptSlug = "first-response"

const DefaultFirstResponsePromptBody = `# PROTOCOLO PRIMER MENSAJE - OBLIGATORIO

Ejecutar los PASOS EN ORDEN. Aplica SIEMPRE, incluso si el mensaje del usuario
es trivial, vacio o basura (ej: "asd", "hola", "x"). No hay excepciones.

## PASO 1 - Llamar 4 tools (en paralelo, ANTES de escribir nada al usuario)
Con project_slug = el slug del bootstrap:
1. ` + "`domain_project_skill_list(project_slug)`" + `
2. ` + "`domain_project_policy_list(project_slug)`" + `
3. ` + "`domain_policy_list()`" + `   (sin argumentos)
4. ` + "`domain_ticket_list(project_slug, limit=5)`" + `

## PASO 2 - Calcular P (proyecto) y G (globales)
- skills:   P = items de la tool 1 con scope=project ; G = items con scope=global
- policies: P = total de la tool 2 ; G = total de la tool 3

## PASO 3 - Emitir EXACTAMENTE este bloque (primera cosa en el mensaje)
` + "```" + `
Buenas. En {project_slug} ({rama}), lo ultimo fue {ultima observation, 1 linea}.

slug:     {project_slug}
rama:     {branch}
remote:   {origin} [{kind}]
head:     {hash[:8]}

skills:   {P} proyecto + {G} globales
policies: {P} proyecto + {G} globales
tickets:  {N} abiertos

ultimo:   {1 linea de la observation mas reciente}
` + "```" + `

## PASO 4 - Solo despues del bloque
Si el usuario pidio una tarea concreta: responderla debajo del bloque.
Si el mensaje fue trivial/vacio: preguntar que necesita, en 1 linea.

## REGLAS DURAS
R1. PROHIBIDO omitir las lineas skills y policies.
R2. PROHIBIDO omitir las globales (G). Siempre se muestran las 2 tools de policies.
R3. PROHIBIDO parafrasear el contexto en prosa o bullets en vez del bloque.
R4. PROHIBIDO responder al usuario antes de emitir el bloque.
R5. PROHIBIDO explicar que tools llamaste o escribir "segun el bootstrap".
R6. PROHIBIDO pasar include_globals a la tool 1 (ya trae globales por default).
R7. El bloque completo: maximo 12 lineas.`

type FirstResponsePromptSeeder struct{}

func (s *FirstResponsePromptSeeder) Name() string    { return "first_response_prompt" }
func (s *FirstResponsePromptSeeder) Version() int    { return 3 }
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
