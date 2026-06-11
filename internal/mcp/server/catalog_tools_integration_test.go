//go:build integration

package mcpserver_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	skillsvc "nunezlagos/domain/internal/service/skill"
)

func TestMCP_SkillExecute(t *testing.T) {
	f := setupMCP(t)
	defer f.cleanup()
	ctx := context.Background()

	_, err := f.skills.Create(ctx, skillsvc.CreateInput{
		OrganizationID: f.orgID, Slug: "greeter", Name: "Greeter",
		SkillType: skillsvc.TypePrompt, Content: "Hola {{name}}",
		InputSchema: map[string]any{
			"type":     "object",
			"required": []any{"name"},
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
			},
		},
		ActorID: f.userID,
	})
	require.NoError(t, err)

	// Ejecución sync con params válidos
	out := callTool(t, f.srv, "domain_skill_execute", map[string]any{
		"skill_slug": "greeter",
		"parameters": map[string]any{"name": "Alice"},
	})
	var res struct {
		Status string `json:"status"`
		Output string `json:"output"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	require.Equal(t, "completed", res.Status)
	require.Contains(t, res.Output, "Alice")

	// Params inválidos (falta required) → error de validación
	_, isErr := callToolRaw(t, f.srv, "domain_skill_execute", map[string]any{
		"skill_slug": "greeter",
		"parameters": map[string]any{},
	})
	require.True(t, isErr)

	// Slug inexistente → error
	_, isErr = callToolRaw(t, f.srv, "domain_skill_execute", map[string]any{
		"skill_slug": "no-existe",
	})
	require.True(t, isErr)
}

func TestMCP_CatalogTools_EndToEnd(t *testing.T) {
	f := setupMCP(t)
	defer f.cleanup()

	// 1. agent_create
	agentOut := callTool(t, f.srv, "domain_agent_create", map[string]any{
		"slug": "mi-agent", "name": "Mi Agent",
		"provider": "anthropic", "model": "claude-sonnet-4-6",
		"system_prompt": "Sos un asistente",
	})
	var ag struct {
		AgentID string `json:"agent_id"`
		Slug    string `json:"slug"`
	}
	require.NoError(t, json.Unmarshal([]byte(agentOut), &ag))
	require.Equal(t, "mi-agent", ag.Slug)
	require.NotEmpty(t, ag.AgentID)

	// agent_create duplicado → error
	_, isErr := callToolRaw(t, f.srv, "domain_agent_create", map[string]any{
		"slug": "mi-agent", "name": "Dup",
		"provider": "anthropic", "model": "claude-sonnet-4-6",
	})
	require.True(t, isErr)

	// 2. flow_create con spec válido
	flowOut := callTool(t, f.srv, "domain_flow_create", map[string]any{
		"slug": "mi-flow", "name": "Mi Flow",
		"spec": map[string]any{
			"version": 1,
			"steps": []any{
				map[string]any{"id": "s1", "type": "condition",
					"config": map[string]any{"expression": "1 == 1"}},
			},
		},
	})
	var fl struct {
		FlowID string `json:"flow_id"`
		Steps  int    `json:"steps"`
	}
	require.NoError(t, json.Unmarshal([]byte(flowOut), &fl))
	require.Equal(t, 1, fl.Steps)

	// flow_create con ciclo → error de validación (sabotaje DAG)
	_, isErr = callToolRaw(t, f.srv, "domain_flow_create", map[string]any{
		"slug": "ciclico", "name": "Cíclico",
		"spec": map[string]any{
			"version": 1,
			"steps": []any{
				map[string]any{"id": "a", "type": "condition", "depends_on": []any{"b"},
					"config": map[string]any{}},
				map[string]any{"id": "b", "type": "condition", "depends_on": []any{"a"},
					"config": map[string]any{}},
			},
		},
	})
	require.True(t, isErr, "spec con ciclo debe ser rechazado")

	// 3. cron_list (vacío)
	cronOut := callTool(t, f.srv, "domain_cron_list", map[string]any{})
	var crons struct {
		Total int `json:"total"`
	}
	require.NoError(t, json.Unmarshal([]byte(cronOut), &crons))
	require.Zero(t, crons.Total)
}
