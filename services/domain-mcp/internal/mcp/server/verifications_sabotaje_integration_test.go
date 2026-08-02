//go:build integration

package mcpserver_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"
)

// resultText vive en el package interno mcpserver, no en este
func textoDelResultado(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	require.NotEmpty(t, res.Content, "un error sin contenido no le dice nada al modelo")
	tc, ok := res.Content[0].(mcp.TextContent)
	require.True(t, ok, "el primer content no es texto")
	return tc.Text
}

// crearCheckpoint agrega un checkpoint de otro kind en el mismo proyecto de la
// fixture, con un único item pendiente.
func crearCheckpoint(t *testing.T, f *verifFixture, kind string) string {
	t.Helper()
	var id string
	require.NoError(t, f.pool.App.QueryRow(context.Background(),
		`INSERT INTO tdd_verifications (project_id, user_id, kind, items, status)
		   SELECT project_id, user_id, $2,
		          '[{"label":"item-a","status":"pending"}]', 'running'
		     FROM tdd_verifications WHERE id = $1
		 RETURNING id`,
		f.verificationID, kind,
	).Scan(&id))
	return id
}

func itemCrudo(t *testing.T, f *verifFixture, label string) map[string]any {
	t.Helper()
	var raw []byte
	require.NoError(t, f.pool.App.QueryRow(context.Background(),
		`SELECT items FROM tdd_verifications WHERE id = $1`, f.verificationID,
	).Scan(&raw))
	var items []map[string]any
	require.NoError(t, json.Unmarshal(raw, &items))
	for _, it := range items {
		if it["label"] == label {
			return it
		}
	}
	t.Fatalf("el label %s no está en los items", label)
	return nil
}

// DOMAINSERV-219 criterio 4: el sabotaje del ciclo TDD vivía como PROSA en el
// template sdd-judge y nadie lo verificaba — update_item aceptaba cerrar un item
// de test en 'pass' sin ninguna evidencia de que el test pueda fallar.
func TestMCP_VerifyUpdateItem_KindTestEnPass_ExigeEvidenciaDeSabotaje(t *testing.T) {
	f := setupVerifConcurrencia(t)
	defer f.cleanup()

	t.Run("sin los campos rechaza y nombra los que faltan", func(t *testing.T) {
		res := callVerifToolResult(t, f.srv, "domain_verify_update_item", map[string]any{
			"verification_id": f.verificationID,
			"project_slug":    "demo",
			"label":           "item-a",
			"status":          "pass",
		})
		require.True(t, res.IsError, "un test 'pass' sin sabotaje es un test que no puede fallar")
		texto := textoDelResultado(t, res)
		for _, campo := range []string{"sabotaje_aplicado", "tests_en_rojo", "restaurado"} {
			require.Contains(t, texto, campo, "el error debe nombrar el campo que falta")
		}
		require.Equal(t, "pending", f.items(t)["item-a"], "el item no se puede mutar si el gate rechazó")
	})

	t.Run("con los campos pasa y los persiste", func(t *testing.T) {
		callVerifTool(t, f.srv, "domain_verify_update_item", map[string]any{
			"verification_id":   f.verificationID,
			"project_slug":      "demo",
			"label":             "item-a",
			"status":            "pass",
			"sabotaje_aplicado": "quité el predicado project_id del SELECT",
			"tests_en_rojo":     "Should be true: un checkpoint de otro proyecto debe ser inaccesible",
			"restaurado":        true,
		})
		item := itemCrudo(t, f, "item-a")
		require.Equal(t, "pass", item["status"])
		require.Equal(t, "quité el predicado project_id del SELECT", item["sabotaje_aplicado"])
		require.Contains(t, item["tests_en_rojo"], "Should be true")
		require.Equal(t, true, item["restaurado"])
	})
}

// La contra-prueba: el gate solo aplica al par (kind='test', status='pass'). Un
// 'fail', un 'skipped' o el kind='policy_review' que usa sdd-review no exigen nada,
// o el flow existente se rompería.
func TestMCP_VerifyUpdateItem_FueraDelParTestPass_NoExigeSabotaje(t *testing.T) {
	f := setupVerifConcurrencia(t)
	defer f.cleanup()

	for _, status := range []string{"fail", "skipped"} {
		callVerifTool(t, f.srv, "domain_verify_update_item", map[string]any{
			"verification_id": f.verificationID,
			"project_slug":    "demo",
			"label":           "item-a",
			"status":          status,
		})
		require.Equal(t, status, f.items(t)["item-a"])
	}

	revisionID := crearCheckpoint(t, f, "policy_review")
	callVerifTool(t, f.srv, "domain_verify_update_item", map[string]any{
		"verification_id": revisionID,
		"project_slug":    "demo",
		"label":           "item-a",
		"status":          "pass",
	})
}
