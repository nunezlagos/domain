//go:build integration

package mcpserver_test

import (
	"context"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/stretchr/testify/require"
)

// callVerifToolResult devuelve el resultado crudo: callVerifTool aborta con
// require.False(IsError) y no sirve para los casos negativos
func callVerifToolResult(t *testing.T, srv *mcptest.Server, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	result, err := srv.Client().CallTool(context.Background(), req)
	require.NoError(t, err)
	return result
}

// el eje de aislamiento es project_id: organization_id se dropeó de projects en la
// migración 000142 (single-tenant, DOMAINSERV-187)
func crearOtroProyecto(t *testing.T, f *verifFixture, slug string) {
	t.Helper()
	_, err := f.pool.App.Exec(context.Background(),
		`INSERT INTO projects (name, slug) VALUES ($1, $2)`, slug, slug)
	require.NoError(t, err)
}

func (f *verifFixture) estado(t *testing.T) (string, bool) {
	t.Helper()
	var status string
	var completadoEn *time.Time
	require.NoError(t, f.pool.App.QueryRow(context.Background(),
		`SELECT status, completed_at FROM tdd_verifications WHERE id = $1`, f.verificationID,
	).Scan(&status, &completadoEn))
	return status, completadoEn != nil
}

// DOMAINSERV-217: update_item y complete resolvían el checkpoint SOLO por UUID, sin
// predicado de proyecto, así que un principal podía mutar y cerrar checkpoints de
// OTRO proyecto (verify_pending sí filtraba: la inconsistencia estaba en el mismo
// archivo). La RLS de tdd_verifications está DESHABILITADA desde la migración
// 000132, así que el guard de aplicación es la ÚNICA barrera.
//
// Los subtests van ordenados: los negativos exigen que el checkpoint siga intacto,
// y el legítimo lo muta al final.
func TestMCP_VerifyTools_CheckpointDeOtroProyecto_NiMutaNiCierra(t *testing.T) {
	f := setupVerifConcurrencia(t)
	defer f.cleanup()
	crearOtroProyecto(t, f, "proyecto-b")

	t.Run("update_item ajeno no muta los items", func(t *testing.T) {
		res := callVerifToolResult(t, f.srv, "domain_verify_update_item", map[string]any{
			"verification_id": f.verificationID,
			"project_slug":    "proyecto-b",
			"label":           "item-a",
			"status":          "pass",
		})
		require.True(t, res.IsError, "un checkpoint de otro proyecto debe ser inaccesible: %+v", res.Content)
		for label, status := range f.items(t) {
			require.Equal(t, "pending", status, "el item %s se mutó desde otro proyecto", label)
		}
	})

	t.Run("complete ajeno no cierra el checkpoint", func(t *testing.T) {
		res := callVerifToolResult(t, f.srv, "domain_verify_complete", map[string]any{
			"verification_id": f.verificationID,
			"project_slug":    "proyecto-b",
		})
		require.True(t, res.IsError, "cerrar un checkpoint ajeno debe fallar: %+v", res.Content)
		status, cerrado := f.estado(t)
		require.Equal(t, "running", status)
		require.False(t, cerrado, "completed_at quedó seteado por un principal de otro proyecto")
	})

	t.Run("sin project_slug falla cerrado", func(t *testing.T) {
		res := callVerifToolResult(t, f.srv, "domain_verify_update_item", map[string]any{
			"verification_id": f.verificationID,
			"label":           "item-a",
			"status":          "pass",
		})
		require.True(t, res.IsError, "escribir sin project_slug es escribir en el proyecto equivocado en silencio")
		res = callVerifToolResult(t, f.srv, "domain_verify_complete", map[string]any{
			"verification_id": f.verificationID,
		})
		require.True(t, res.IsError, "cerrar sin project_slug es fail-closed")
	})

	// el dueño legítimo no se rompe. status='fail' a propósito: el gate de sabotaje
	// (DOMAINSERV-219) solo aplica a kind='test' + status='pass' y este test es de authz
	t.Run("el proyecto dueño muta y cierra", func(t *testing.T) {
		callVerifTool(t, f.srv, "domain_verify_update_item", map[string]any{
			"verification_id": f.verificationID,
			"project_slug":    "demo",
			"label":           "item-a",
			"status":          "fail",
		})
		require.Equal(t, "fail", f.items(t)["item-a"])

		callVerifTool(t, f.srv, "domain_verify_complete", map[string]any{
			"verification_id": f.verificationID,
			"project_slug":    "demo",
		})
		status, cerrado := f.estado(t)
		require.Equal(t, "failed", status)
		require.True(t, cerrado, "completed_at debe quedar seteado")
	})
}
