//go:build integration

// DOMAINSERV-218, incremento 3 — criterio 3 del ticket contra una base real.
//
// POR QUÉ ESTE ARCHIVO EXISTE Y NO ALCANZAN LOS UNITARIOS QUE YA HAY.
// SolapamientoConOtros, HayScopesDeOtros y ValidarParticionDisjunta están cubiertos por 18 tests
// de lógica pura, pero TODOS reciben los scopes vigentes ya materializados. El camino que los
// alimenta —la migración 000289, el UPSERT por (flow_run_id, agent_id), el INSERT ... SELECT que
// deriva el project_id del flow_run y el RLS FORCE con el GUC app.current_project_id— no lo
// ejercitaba nada.
//
// Ese hueco no es teórico: con FORCE ROW LEVEL SECURITY, si el GUC no queda seteado en la tx, la
// query de scopes vigentes devuelve CERO FILAS SIN ERROR. El guard de solapamiento pasaría en
// verde siempre y el criterio 3 estaría roto exactamente igual que cuando ValidarParticionDisjunta
// no tenía callers, pero ahora con 18 tests unitarios en verde tapándolo.
package mcpserver_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/mcptest"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/db"
	mcpserver "nunezlagos/domain/internal/mcp/server"
	dmigrate "nunezlagos/domain/internal/migrate"
	"nunezlagos/domain/internal/seeds"
	flowsvc "nunezlagos/domain/internal/service/flow"
	"nunezlagos/domain/internal/service/orchestrator"
	"nunezlagos/domain/internal/service/orchestrator/phases"
)

type scopesFixture struct {
	srv       *mcptest.Server
	projectID string
	cleanup   func()
}

// setupFlowScopesMCP levanta el server MCP con Pool y FlowToken inyectados. El fixture de
// orchestrate_tools_integration_test.go no sirve acá: no pasa Pool, así que withOrgTxHandler no
// abre tx, reservarTerritorio encuentra txctx nil y se saltea la persistencia entera — el test
// pasaría sin tocar una sola fila.
func setupFlowScopesMCP(t *testing.T) *scopesFixture {
	t.Helper()
	ctx := context.Background()
	pgC, err := postgres.Run(ctx,
		"pgvector/pgvector:pg16",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	require.NoError(t, err)
	dsn, _ := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, dmigrate.Up(dsn))

	pools, err := db.OpenWithRoleOverride(ctx, dsn, "app_user", "app_admin")
	require.NoError(t, err)

	rec := &audit.PGRecorder{Pool: pools.Auth}
	org, owner, err := seedOrgUser(ctx, pools.App, "Acme", "acme", "owner@acme.com", "Owner")
	require.NoError(t, err)
	_, err = seeds.SeedAgentTemplatesForOrg(ctx, pools.App, org.ID)
	require.NoError(t, err)
	_, err = seeds.SeedFlowsForOrg(ctx, pools.App, org.ID)
	require.NoError(t, err)

	var projectID string
	require.NoError(t, pools.App.QueryRow(ctx,
		`INSERT INTO projects (name, slug) VALUES ('Demo', 'demo') RETURNING id`,
	).Scan(&projectID))

	reg := phases.NewRegistry()
	reg.MustRegister(phases.NewSDDApplyHandler())
	reg.MustRegister(phases.NewSDDVerifyHandler())
	orchSvc := orchestrator.New(pools.App, rec, reg, "dev")

	deps := mcpserver.Deps{
		Principal: &apikey.Principal{
			UserID:         owner.UserID.String(),
			OrganizationID: org.ID.String(),
			Role:           "owner",
		},
		Orchestrator: orchSvc,
		Pool:         pools.App,
		FlowToken:    flowsvc.NewFlowTokenService([]byte("secreto-de-test-para-flow-tokens")),
		ServerName:   "domain-mcp-test",
		ServerVer:    "0.0.0",
	}
	testSrv, err := mcptest.NewServer(t, mcpserver.Tools(deps)...)
	require.NoError(t, err)

	return &scopesFixture{
		srv:       testSrv,
		projectID: projectID,
		cleanup: func() {
			testSrv.Close()
			pools.Close()
			_ = pgC.Terminate(ctx)
		},
	}
}

// nuevoFlowRun crea un flow_run running: grant_token exige un flow activo antes de mirar
// territorio, así que sin esto el test mediría el check de flow y no el de solapamiento.
func (f *scopesFixture) nuevoFlowRun(t *testing.T, texto string) string {
	t.Helper()
	txt := callOrchTool(t, f.srv, "domain_orchestrate", map[string]any{
		"raw_text":   texto,
		"mode":       "express",
		"project_id": f.projectID,
	})
	var res struct {
		FlowRunID string `json:"flow_run_id"`
	}
	require.NoError(t, json.Unmarshal([]byte(txt), &res))
	require.NotEmpty(t, res.FlowRunID)
	return res.FlowRunID
}

// grantToken llama a la tool devolviendo también si fue error, porque acá el rechazo ES el
// resultado esperado en la mitad de los casos y callOrchTool aborta ante IsError.
func grantToken(t *testing.T, srv *mcptest.Server, flowRunID, agentID string, allowedPaths ...string) (string, bool) {
	t.Helper()
	args := map[string]any{
		"flow_run_id": flowRunID,
		"session_id":  "sesion-compartida-por-los-subagentes",
	}
	if agentID != "" {
		args["agent_id"] = agentID
	}
	if len(allowedPaths) > 0 {
		paths := make([]any, 0, len(allowedPaths))
		for _, p := range allowedPaths {
			paths = append(paths, p)
		}
		args["allowed_paths"] = paths
	}
	req := mcp.CallToolRequest{}
	req.Params.Name = "domain_flow_grant_token"
	req.Params.Arguments = args
	result, err := srv.Client().CallTool(context.Background(), req)
	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	tc, ok := result.Content[0].(mcp.TextContent)
	require.True(t, ok, "expected text content")
	return tc.Text, result.IsError
}

func tokenDelGrant(t *testing.T, txt string) string {
	t.Helper()
	var res struct {
		Token   string `json:"token"`
		AgentID string `json:"agent_id"`
	}
	require.NoError(t, json.Unmarshal([]byte(txt), &res))
	return res.Token
}

// Criterio 3 del ticket: el solapamiento se rechaza al EMITIR, no al editar. Es el escenario que
// ValidarParticionDisjunta no podía cumplir mientras no tuviera contra qué comparar.
func TestFlowGrantToken_AllowlistSolapadaConOtroAgente_RechazaYNoEmite(t *testing.T) {
	f := setupFlowScopesMCP(t)
	defer f.cleanup()

	flowRunID := f.nuevoFlowRun(t, "dos subagentes que se pisan")

	txtA, errA := grantToken(t, f.srv, flowRunID, "agente-a", "services/domain-mcp/**")
	require.False(t, errA, "el primer grant no tiene con quién solaparse: %s", txtA)
	require.NotEmpty(t, tokenDelGrant(t, txtA))

	// internal/** está DENTRO de services/domain-mcp/**: es solapamiento por ancestro, no por
	// igualdad, que es el caso que un chequeo ingenuo por string se pierde.
	txtB, errB := grantToken(t, f.srv, flowRunID, "agente-b", "services/domain-mcp/internal/**")
	require.True(t, errB, "el segundo grant debe rechazarse; devolvió: %s", txtB)
	require.Contains(t, txtB, "solapan", "el deny tiene que nombrar el conflicto, no fallar genérico")
	require.NotContains(t, txtB, `"token"`, "un grant rechazado NO puede devolver token")
}

// MUST-5: cada cierre de fase re-emite el token (claude_hook.go:41 dispara el grant en
// orchestrate_phase_result, flow_status y orchestrate_confirm). Si la fila fuera por emisión, o si
// el chequeo no excluyera la propia, el agente se bloquearía a sí mismo en la segunda fase de
// cualquier flow real. Contra la base es donde se ve: es el UPSERT el que lo evita.
func TestFlowGrantToken_MismoAgenteRenovandoSuScope_NoSeAutoBloquea(t *testing.T) {
	f := setupFlowScopesMCP(t)
	defer f.cleanup()

	flowRunID := f.nuevoFlowRun(t, "un agente que renueva al cerrar cada fase")

	primero, err1 := grantToken(t, f.srv, flowRunID, "agente-a", "services/domain-mcp/**")
	require.False(t, err1, "primer grant: %s", primero)

	segundo, err2 := grantToken(t, f.srv, flowRunID, "agente-a", "services/domain-mcp/**")
	require.False(t, err2, "la renovación del MISMO agente con el MISMO scope no puede rechazarse: %s", segundo)
	require.NotEmpty(t, tokenDelGrant(t, segundo))

	// mover el propio scope tampoco se auto-bloquea: el subconjunto solapa con la fila anterior
	// del propio agente, y esa es justamente la que hay que excluir
	tercero, err3 := grantToken(t, f.srv, flowRunID, "agente-a", "services/domain-mcp/internal/**")
	require.False(t, err3, "el agente que reduce su propio scope tampoco se auto-bloquea: %s", tercero)
}

// Criterio 1 del ticket a nivel de emisión: dos agentes con territorio disjunto obtienen cada uno
// su token, sin desactivar ni relajar nada.
func TestFlowGrantToken_DosAgentesConScopesDisjuntos_LosDosEmiten(t *testing.T) {
	f := setupFlowScopesMCP(t)
	defer f.cleanup()

	flowRunID := f.nuevoFlowRun(t, "dos subagentes con territorios separados")

	txtA, errA := grantToken(t, f.srv, flowRunID, "agente-a", "services/domain-mcp/**")
	require.False(t, errA, "agente-a: %s", txtA)
	txtB, errB := grantToken(t, f.srv, flowRunID, "agente-b", "install-user/**")
	require.False(t, errB, "agente-b: %s", txtB)

	tokenA := tokenDelGrant(t, txtA)
	tokenB := tokenDelGrant(t, txtB)
	require.NotEmpty(t, tokenA)
	require.NotEmpty(t, tokenB)
	require.NotEqual(t, tokenA, tokenB, "dos agentes de la MISMA sesión no pueden compartir token: es el bug que el incremento 3 cierra")
}

// El hilo principal ('' como agent_id) es un ocupante más de la tabla: si reservó territorio, un
// subagente que lo reclame se rechaza igual. Sin esto el eje de agente tendría un agujero por
// donde el caso más común —el flow que arranca sin subagentes— quedaría fuera del guard.
func TestFlowGrantToken_HiloPrincipalYaReservoTerritorio_ElSubagenteQueLoReclamaEsRechazado(t *testing.T) {
	f := setupFlowScopesMCP(t)
	defer f.cleanup()

	flowRunID := f.nuevoFlowRun(t, "el hilo principal reserva y despues delega")

	txtP, errP := grantToken(t, f.srv, flowRunID, "", "services/domain-mcp/**")
	require.False(t, errP, "el hilo principal emite normal: %s", txtP)

	txtS, errS := grantToken(t, f.srv, flowRunID, "agente-a", "services/domain-mcp/internal/**")
	require.True(t, errS, "el subagente no puede reclamar lo que el hilo principal ya tiene; devolvió: %s", txtS)
	require.Contains(t, txtS, "solapan")
}

// El scope se reserva POR FLOW: dos flows distintos pueden pedir el mismo territorio sin chocar.
// Si el chequeo mirara todas las filas y no solo las del flow_run, un flow viejo bloquearía a uno
// nuevo y el gate quedaría insatisfacible con el tiempo — el modo de falla que empuja al bypass.
func TestFlowGrantToken_MismoScopeEnOtroFlowRun_NoHayConflicto(t *testing.T) {
	f := setupFlowScopesMCP(t)
	defer f.cleanup()

	flowUno := f.nuevoFlowRun(t, "primer flow")
	flowDos := f.nuevoFlowRun(t, "segundo flow")
	require.NotEqual(t, flowUno, flowDos)

	txt1, err1 := grantToken(t, f.srv, flowUno, "agente-a", "services/domain-mcp/**")
	require.False(t, err1, "flow uno: %s", txt1)

	txt2, err2 := grantToken(t, f.srv, flowDos, "agente-b", "services/domain-mcp/**")
	require.False(t, err2, "el mismo territorio en OTRO flow no es conflicto: %s", txt2)
}
