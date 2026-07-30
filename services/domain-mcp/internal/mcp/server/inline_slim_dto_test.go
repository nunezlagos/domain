package mcpserver

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	agentsvc "nunezlagos/domain/internal/service/agent"
	capturedpromptsvc "nunezlagos/domain/internal/service/capturedprompt"
	flowsvc "nunezlagos/domain/internal/service/flow"
	projectpolicysvc "nunezlagos/domain/internal/service/projectpolicy"
	skillsvc "nunezlagos/domain/internal/service/skill"
)

// Los tres structs de origen no declaran tags json, así que sus keys salen en
// PascalCase. Estos tests fijan que el cuerpo desaparezca SIN que se renombre
// ninguna otra key: el alcance de DOMAINSERV-161 es el derrame, no el naming.

func TestSkillSlim_Proyectar_ContentLargo_OmiteCuerpoYExponeLen(t *testing.T) {
	cuerpo := strings.Repeat("x", 5000)
	s := skillsvc.Skill{ID: uuid.New(), Slug: "una-skill", Name: "Una skill", Content: cuerpo}

	b, err := json.Marshal(proyectarSkillSlim(&s))
	require.NoError(t, err)
	out := string(b)

	require.NotContains(t, out, `"Content"`)
	require.NotContains(t, out, strings.Repeat("x", 100))
	require.Contains(t, out, `"ContentLen":5000`)
	require.Contains(t, out, `"Slug":"una-skill"`)
	require.Contains(t, out, s.ID.String())
}

func TestAgentSlim_Proyectar_SystemPromptLargo_OmiteCuerpoYExponeLen(t *testing.T) {
	prompt := strings.Repeat("y", 4000)
	a := agentsvc.Agent{ID: uuid.New(), Slug: "un-agente", SystemPrompt: prompt}

	b, err := json.Marshal(proyectarAgentSlim(&a))
	require.NoError(t, err)
	out := string(b)

	require.NotContains(t, out, `"SystemPrompt"`)
	require.NotContains(t, out, strings.Repeat("y", 100))
	require.Contains(t, out, `"SystemPromptLen":4000`)
	require.Contains(t, out, `"Slug":"un-agente"`)
}

func TestFlowSlim_Proyectar_SpecConSteps_OmiteSpecYExponeConteo(t *testing.T) {
	f := flowsvc.Flow{
		ID:   uuid.New(),
		Slug: "un-flow",
		Spec: flowsvc.Spec{
			Version: 3,
			Steps: []flowsvc.Step{
				{ID: "paso-uno", Type: "skill", Config: map[string]any{"clave": strings.Repeat("z", 500)}},
				{ID: "paso-dos", Type: "skill"},
			},
		},
	}

	b, err := json.Marshal(proyectarFlowSlim(&f))
	require.NoError(t, err)
	out := string(b)

	require.NotContains(t, out, `"Spec"`)
	require.NotContains(t, out, "paso-uno")
	require.NotContains(t, out, strings.Repeat("z", 100))
	require.Contains(t, out, `"SpecVersion":3`)
	require.Contains(t, out, `"SpecStepsCount":2`)
	require.Contains(t, out, `"Slug":"un-flow"`)
}

func TestProyectarParaListado_ListaConNils_SaltaLosNils(t *testing.T) {
	s := skillsvc.Skill{Slug: "viva", Content: "algo"}

	require.Len(t, proyectarSkillsParaListado([]skillsvc.Skill{s, s}), 2)
	require.Len(t, proyectarAgentsParaListado([]agentsvc.Agent{{Slug: "a"}}), 1)
	require.Len(t, proyectarFlowsParaListado([]flowsvc.Flow{{Slug: "f"}}), 1)
	require.Empty(t, proyectarSkillsParaListado(nil))
}

func TestPolicySlim_Proyectar_BodyLargo_OmiteCuerpoYConservaOverridePlatform(t *testing.T) {
	cuerpo := strings.Repeat("w", 3000)
	p := projectpolicysvc.Policy{
		ID:               uuid.New(),
		Slug:             "una-policy",
		Name:             "Una policy",
		Kind:             "convention",
		BodyMD:           cuerpo,
		BodyStructured:   map[string]any{"reglas": strings.Repeat("v", 800)},
		Version:          4,
		OverridePlatform: true,
	}

	b, err := json.Marshal(proyectarPolicySlim(&p))
	require.NoError(t, err)
	out := string(b)

	require.NotContains(t, out, `"body_md":"`)
	require.NotContains(t, out, `"body_structured"`)
	require.NotContains(t, out, strings.Repeat("w", 100))
	require.NotContains(t, out, strings.Repeat("v", 100))
	require.Contains(t, out, `"body_md_len":3000`)
	// sdd-review depende de este campo para decidir si la policy de proyecto pisa a la de plataforma
	require.Contains(t, out, `"override_platform":true`)
	require.Contains(t, out, `"slug":"una-policy"`)
	require.Contains(t, out, `"kind":"convention"`)
	require.Contains(t, out, `"version":4`)
}

// CharCount ya venía en el struct de origen: el test fija que se reusa y no se duplica
func TestPromptCapturadoSlim_Proyectar_ContentLargo_OmiteCuerpoYReusaCharCount(t *testing.T) {
	p := capturedpromptsvc.Prompt{ID: uuid.New(), Content: strings.Repeat("u", 2500), CharCount: 2500}

	b, err := json.Marshal(proyectarPromptsCapturadosParaListado([]*capturedpromptsvc.Prompt{&p, nil}))
	require.NoError(t, err)
	out := string(b)

	require.NotContains(t, out, `"content":"`)
	require.NotContains(t, out, strings.Repeat("u", 100))
	require.Contains(t, out, `"char_count":2500`)
	require.NotContains(t, out, "content_len")
}

// nil devuelve zero value, no un puntero que serializaría "null"
func TestProyectarSlim_Nil_DevuelveZeroValue(t *testing.T) {
	require.Equal(t, 0, proyectarSkillSlim(nil).ContentLen)
	require.Equal(t, 0, proyectarAgentSlim(nil).SystemPromptLen)
	require.Equal(t, 0, proyectarFlowSlim(nil).SpecStepsCount)
}
