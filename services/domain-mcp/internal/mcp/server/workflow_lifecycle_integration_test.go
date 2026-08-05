//go:build integration

package mcpserver_test

import (
	"context"
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
