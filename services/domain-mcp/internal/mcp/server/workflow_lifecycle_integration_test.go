//go:build integration

package mcpserver_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/observability"
)

// DOMAINSERV-230: el ciclo de vida de workflows no cerraba. 'completed' y 'failed'
// estaban declarados y sin un solo uso en el repo, el ON CONFLICT no tocaba status
// ni ended_at, y started_at salía de un reloj distinto que ended_at. Medido en prod:
// 20 de 20 filas en 'abandoned', 8 con ended_at ANTERIOR a last_activity_at y varias
// con ended_at anterior a su propio started_at.
//
// Estos tests van contra Postgres real porque los tres defectos vivían en el SQL del
// upsert: un stub del store no los habría visto nunca.

func storeParaLifecycle(t *testing.T) (*observability.PGWorkflowStore, func()) {
	t.Helper()
	pools, terminar := pgMigradoParaWorkflows(t, context.Background())
	return &observability.PGWorkflowStore{Pool: pools.App}, func() {
		pools.Close()
		terminar()
	}
}

// El caso que obliga a que el cierre sea un Upsert y no un UPDATE:
// conWorkflowDeLaCorrida no envuelve domain_orchestrate ni orchestrate_phase_result,
// así que el cierre puede llegar antes de que exista la fila. Un UPDATE pelado
// no-operaría en silencio y el workflow quedaría sin cerrar para siempre.
func TestCloseWorkflow_CierreAntesDeLaFila_LaCreaCerrada(t *testing.T) {
	store, cleanup := storeParaLifecycle(t)
	defer cleanup()
	ctx := context.Background()

	id := uuid.New()
	fin := time.Now().UTC().Truncate(time.Millisecond)
	require.NoError(t, store.CloseWorkflow(ctx, id, "completed", fin))

	w, err := store.GetWorkflow(ctx, id)
	require.NoError(t, err)
	require.Equal(t, observability.WorkflowCompleted, w.Status,
		"un cierre que llega antes de la fila tiene que crearla, no perderse")
	require.NotNil(t, w.EndedAt)
	require.False(t, w.StartedAt.After(*w.EndedAt),
		"started_at no puede ser posterior a ended_at: eso es una duración negativa")
}

func TestCloseWorkflow_SobreFilaRunning_LaCierra(t *testing.T) {
	store, cleanup := storeParaLifecycle(t)
	defer cleanup()
	ctx := context.Background()

	id := uuid.New()
	require.NoError(t, store.UpsertWorkflow(ctx, observability.WorkflowRow{
		ID: id, Status: observability.WorkflowRunning, TotalToolCalls: 1,
		LastActivityAt: time.Now().UTC(),
	}))

	require.NoError(t, store.CloseWorkflow(ctx, id, "failed", time.Now().UTC()))

	w, err := store.GetWorkflow(ctx, id)
	require.NoError(t, err)
	require.Equal(t, observability.WorkflowFailed, w.Status)
	require.NotNil(t, w.EndedAt)
	require.Equal(t, 1, w.TotalToolCalls, "el cierre no aporta tool calls: no debe sumar")
}

// El estado 'cancelled' es el motivo de la migración 000283: colapsarlo en 'failed'
// borraría la distinción entre "falló" y "lo canceló un humano".
func TestCloseWorkflow_CancelledSobrevive_NoColapsaEnFailed(t *testing.T) {
	store, cleanup := storeParaLifecycle(t)
	defer cleanup()
	ctx := context.Background()

	id := uuid.New()
	require.NoError(t, store.CloseWorkflow(ctx, id, "cancelled", time.Now().UTC()))

	w, err := store.GetWorkflow(ctx, id)
	require.NoError(t, err)
	require.Equal(t, observability.WorkflowCancelled, w.Status)
}

func TestCloseWorkflow_StatusNoTerminal_EsRechazado(t *testing.T) {
	store, cleanup := storeParaLifecycle(t)
	defer cleanup()
	ctx := context.Background()

	id := uuid.New()
	require.Error(t, store.CloseWorkflow(ctx, id, "running", time.Now().UTC()),
		"'running' no es un cierre: aceptarlo dejaría la fila abierta creyendo que cerró")
	require.Error(t, store.CloseWorkflow(ctx, id, "cualquier-cosa", time.Now().UTC()))
}

// El defecto #2 del ticket: MarkWorkflowIdle filtra WHERE status='running', así que
// una fila que el reaper dio por muerta no volvía a 'running' por más Touch que
// recibiera — quedaba con su ended_at congelado mientras seguía consumiendo tool
// calls. En prod: 4a0cfd08 siguió recibiendo actividad 9h 54min después de su cierre.
func TestUpsertWorkflow_FilaAbandonada_ReviveConActividadNueva(t *testing.T) {
	store, cleanup := storeParaLifecycle(t)
	defer cleanup()
	ctx := context.Background()

	id := uuid.New()
	require.NoError(t, store.UpsertWorkflow(ctx, observability.WorkflowRow{
		ID: id, Status: observability.WorkflowRunning, TotalToolCalls: 1,
		LastActivityAt: time.Now().UTC().Add(-time.Hour),
	}))

	n, err := store.MarkWorkflowIdle(ctx, time.Minute)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	muerta, err := store.GetWorkflow(ctx, id)
	require.NoError(t, err)
	require.Equal(t, observability.WorkflowAbandoned, muerta.Status)
	require.NotNil(t, muerta.EndedAt)

	// Llega actividad nueva: la fila estaba viva, el reaper se equivocó.
	require.NoError(t, store.UpsertWorkflow(ctx, observability.WorkflowRow{
		ID: id, Status: observability.WorkflowRunning, TotalToolCalls: 1,
		LastActivityAt: time.Now().UTC(),
	}))

	viva, err := store.GetWorkflow(ctx, id)
	require.NoError(t, err)
	require.Equal(t, observability.WorkflowRunning, viva.Status,
		"una fila que vuelve a recibir actividad no puede seguir figurando como muerta")
	require.Nil(t, viva.EndedAt,
		"si revive, su ended_at anterior es mentira: tiene que volver a NULL")
	require.Equal(t, 2, viva.TotalToolCalls)
}

// La contracara: un workflow YA cerrado no se reabre por un Touch tardío. Si no,
// cualquier llamada rezagada resucitaría una corrida terminada.
func TestUpsertWorkflow_FilaCerrada_NoLaReabreUnTouchTardio(t *testing.T) {
	store, cleanup := storeParaLifecycle(t)
	defer cleanup()
	ctx := context.Background()

	id := uuid.New()
	require.NoError(t, store.CloseWorkflow(ctx, id, "completed", time.Now().UTC()))

	require.NoError(t, store.UpsertWorkflow(ctx, observability.WorkflowRow{
		ID: id, Status: observability.WorkflowRunning, TotalToolCalls: 1,
		LastActivityAt: time.Now().UTC(),
	}))

	w, err := store.GetWorkflow(ctx, id)
	require.NoError(t, err)
	require.Equal(t, observability.WorkflowCompleted, w.Status,
		"'completed' es terminal: un touch rezagado no lo revierte")
	require.NotNil(t, w.EndedAt)
}

// La contracara de la reapertura, y el caso que la refutación adversarial exigió:
// una fila 'abandoned' NO puede revivir si su corrida ya cerró. Sin el NOT EXISTS
// contra flow_runs, una tool call rezagada resucita como 'running' el workflow de un
// flow_run terminal — peor que el 'abandoned' que la reapertura venía a arreglar,
// porque inventa una corrida viva que no existe.
func TestUpsertWorkflow_CorridaYaTerminal_NoRevivePorMasQueLlegueActividad(t *testing.T) {
	srv, projectID, store, cleanup := setupWorkflowMCP(t)
	defer cleanup()
	ctx := context.Background()

	startTxt := callOrchTool(t, srv, "domain_orchestrate", map[string]any{
		"raw_text":   "fix typo en README",
		"mode":       "express",
		"project_id": projectID,
	})
	var start struct {
		FlowRunID string `json:"flow_run_id"`
	}
	require.NoError(t, json.Unmarshal([]byte(startTxt), &start))
	flowRunID := uuid.MustParse(start.FlowRunID)

	// una tool call declara la corrida, así nace la fila de workflow
	callOrchTool(t, srv, "domain_flow_status", map[string]any{"flow_run_id": start.FlowRunID})

	// el reaper la da por muerta
	_, err := store.Pool.Exec(ctx,
		`UPDATE workflows SET last_activity_at = now() - interval '1 hour' WHERE id = $1`, flowRunID)
	require.NoError(t, err)
	n, err := store.MarkWorkflowIdle(ctx, time.Minute)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	// y la corrida cierra de verdad
	_, err = store.Pool.Exec(ctx,
		`UPDATE flow_runs SET status = 'completed', finished_at = now() WHERE id = $1`, flowRunID)
	require.NoError(t, err)

	// llega una tool call rezagada
	require.NoError(t, store.UpsertWorkflow(ctx, observability.WorkflowRow{
		ID: flowRunID, Status: observability.WorkflowRunning, TotalToolCalls: 1,
		LastActivityAt: time.Now().UTC(),
	}))

	w, err := store.GetWorkflow(ctx, flowRunID)
	require.NoError(t, err)
	require.Equal(t, observability.WorkflowAbandoned, w.Status,
		"la corrida ya cerró: una tool call rezagada no puede resucitar el workflow como running")
	require.NotNil(t, w.EndedAt, "y su ended_at no puede volver a NULL")
}

// El defecto #3: dos relojes. started_at caía en el now() de Postgres al ejecutar el
// statement y last_activity_at venía del time.Now() de Go calculado antes de
// despachar la query, y MarkWorkflowIdle hace ended_at = last_activity_at. Para un
// workflow de un solo touch, started_at salía SIEMPRE posterior a ended_at, y
// siempre con el mismo signo (−25 ms, −3 ms): orden de ejecución, no skew.
func TestUpsertWorkflow_UnSoloTouch_LaDuracionNoEsNegativa(t *testing.T) {
	store, cleanup := storeParaLifecycle(t)
	defer cleanup()
	ctx := context.Background()

	id := uuid.New()
	require.NoError(t, store.UpsertWorkflow(ctx, observability.WorkflowRow{
		ID: id, Status: observability.WorkflowRunning, TotalToolCalls: 1,
		LastActivityAt: time.Now().UTC(),
	}))

	n, err := store.MarkWorkflowIdle(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	w, err := store.GetWorkflow(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, w.EndedAt)
	require.False(t, w.StartedAt.After(*w.EndedAt),
		"un workflow de un solo touch no puede durar menos que cero: started_at=%s ended_at=%s",
		w.StartedAt, *w.EndedAt)
}

// El invariante que declara la migración 000284. Va como NOT VALID, así que no revisa el
// histórico — pero SÍ tiene que rechazar toda fila nueva. Un constraint declarado que no
// rechaza nada es la versión SQL de un guard que no se ejecuta.
func TestWorkflows_ElInvarianteRechazaUnaDuracionNegativa(t *testing.T) {
	store, cleanup := storeParaLifecycle(t)
	defer cleanup()
	ctx := context.Background()

	ahora := time.Now().UTC()
	_, err := store.Pool.Exec(ctx, `
		INSERT INTO workflows (id, status, started_at, ended_at, last_activity_at, metadata)
		VALUES ($1, 'completed', $2, $3, $2, '{}'::jsonb)`,
		uuid.New(), ahora, ahora.Add(-time.Minute))
	require.Error(t, err, "una duración negativa no puede entrar a la tabla")
	require.Contains(t, err.Error(), "workflows_ended_after_started",
		"y el que la rechaza tiene que ser el constraint del invariante, no otra cosa")
}

// La contracara: ended_at NULL está permitido a propósito, porque es una corrida VIVA. Si
// el constraint lo rechazara, ningún workflow podría abrirse.
func TestWorkflows_ElInvarianteAceptaEndedAtNulo(t *testing.T) {
	store, cleanup := storeParaLifecycle(t)
	defer cleanup()
	ctx := context.Background()

	require.NoError(t, store.UpsertWorkflow(ctx, observability.WorkflowRow{
		ID: uuid.New(), Status: observability.WorkflowRunning,
		LastActivityAt: time.Now().UTC(),
	}), "una corrida viva no tiene ended_at y tiene que poder existir")
}

// DOMAINSERV-229, camino de lectura contra la BD real: un workflow sin actor tiene
// esas columnas en NULL, y el row tiene que devolverlas como nil y no como el uuid
// en ceros. Con los campos no-puntero, un NULL era indistinguible del centinela.
func TestGetWorkflow_ColumnasNulas_VuelvenNilNoElCentinela(t *testing.T) {
	store, cleanup := storeParaLifecycle(t)
	defer cleanup()
	ctx := context.Background()

	id := uuid.New()
	require.NoError(t, store.UpsertWorkflow(ctx, observability.WorkflowRow{
		ID: id, Status: observability.WorkflowRunning, LastActivityAt: time.Now().UTC(),
	}))

	w, err := store.GetWorkflow(ctx, id)
	require.NoError(t, err)
	require.Nil(t, w.ActorID)
	require.Nil(t, w.APIKeyID)
	require.Nil(t, w.ProjectID)
}

// Y el complemento: un puntero a uuid.Nil tampoco debe llegar a la columna. La
// normalización vive en el borde de la escritura (nullableUUIDPtr) para que no
// dependa de la disciplina de cada caller.
func TestUpsertWorkflow_PunteroAlUUIDCero_PersisteNULL(t *testing.T) {
	store, cleanup := storeParaLifecycle(t)
	defer cleanup()
	ctx := context.Background()

	id := uuid.New()
	cero := uuid.Nil
	require.NoError(t, store.UpsertWorkflow(ctx, observability.WorkflowRow{
		ID: id, Status: observability.WorkflowRunning, LastActivityAt: time.Now().UTC(),
		ActorID: &cero, APIKeyID: &cero, ProjectID: &cero,
	}))

	var actor, apiKey *uuid.UUID
	require.NoError(t, store.Pool.QueryRow(ctx,
		`SELECT actor_id, api_key_id FROM workflows WHERE id = $1`, id).Scan(&actor, &apiKey))
	require.Nil(t, actor, "el centinela no puede entrar a la columna ni por un puntero")
	require.Nil(t, apiKey)
}
