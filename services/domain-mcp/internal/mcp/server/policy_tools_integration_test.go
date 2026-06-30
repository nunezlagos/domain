//go:build integration

package mcpserver_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	policysvc "nunezlagos/domain/internal/service/policy"
)

func TestMCP_PolicyGetAndList(t *testing.T) {
	f := setupMCP(t)
	defer f.cleanup()
	ctx := context.Background()

	_, err := f.policies.Create(ctx, policysvc.CreateInput{
		Slug: "go", Name: "Go Conventions", Kind: policysvc.KindConvention,
		BodyMD: "# Go\n\n- Usa pgx v5 para Postgres\n- Errores con %w siempre",
	})
	require.NoError(t, err)
	_, err = f.policies.Create(ctx, policysvc.CreateInput{
		Slug: "testing", Name: "Testing Conventions", Kind: policysvc.KindConvention,
		BodyMD: "# Testing\n\n- TDD estricto",
	})
	require.NoError(t, err)

	out := callTool(t, f.srv, "domain_policy_get", map[string]any{"slug": "go"})
	var p struct {
		Slug    string `json:"slug"`
		Kind    string `json:"kind"`
		Version int    `json:"version"`
		BodyMD  string `json:"body_md"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &p))
	require.Equal(t, "go", p.Slug)
	require.Equal(t, 1, p.Version)
	require.Contains(t, p.BodyMD, "pgx v5")

	out = callTool(t, f.srv, "domain_policy_list", map[string]any{})
	var list struct {
		Total    int `json:"total"`
		Policies []struct {
			Slug string `json:"slug"`
		} `json:"policies"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &list))
	require.Equal(t, 2, list.Total)

	_, isErr := callToolRaw(t, f.srv, "domain_policy_get", map[string]any{"slug": "nope"})
	require.True(t, isErr)
}
