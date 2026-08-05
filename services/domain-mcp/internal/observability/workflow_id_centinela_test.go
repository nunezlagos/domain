package observability

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// DOMAINSERV-229: cuatro de los cinco writers de observabilidad convertían el
// workflow del ctx con wfID.String() en vez del guard workflowIDForRow. La
// diferencia importa porque uuid.Nil.String() devuelve
// "00000000-0000-0000-0000-000000000000", que NO es cadena vacía: el NULLIF($n,'')
// de cada INSERT nunca disparaba y el centinela terminaba persistido en una
// columna NULLable. Un centinela indistinguible de un dato real, y los índices
// parciales sobre esa columna llenándose de basura en vez de quedar vacíos.
//
// Los tests que ya existían verificaban el ctx y el header, pero NINGUNO asertaba
// el campo que se persiste, así que el defecto era invisible para la suite. Estos
// cierran ese hueco por test y no por disciplina: cada uno de los 4 writers tiene
// su par sin-workflow / con-workflow.

func TestHTTPLogger_SinWorkflow_NoPersisteElCentinela(t *testing.T) {
	store := &stubHTTPLogStore{}
	h := NewHTTPLogger(store, nil, 1)

	mw := h.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	mw.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))
	h.Close()

	require.Len(t, store.logs, 1)
	require.Empty(t, store.logs[0].WorkflowID,
		"sin workflow el campo va vacío para que el NULLIF del INSERT deje NULL; el uuid en ceros es un dato falso")
}

func TestHTTPLogger_ConWorkflow_PersisteElUUIDReal(t *testing.T) {
	store := &stubHTTPLogStore{}
	h := NewHTTPLogger(store, nil, 1)

	quiero := uuid.New()
	mw := h.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Workflow-Id", quiero.String())
	mw.ServeHTTP(httptest.NewRecorder(), req)
	h.Close()

	require.Len(t, store.logs, 1)
	require.Equal(t, quiero.String(), store.logs[0].WorkflowID,
		"el guard no puede tragarse un workflow real: solo colapsa el Nil")
}

func TestSlowQueryTracer_SinWorkflow_NoPersisteElCentinela(t *testing.T) {
	store := &stubSlowStore{}
	tr := NewSlowQueryTracer(&testInnerTracer{}, store, nil, 1, 0)

	ctx := tr.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{SQL: "SELECT 1"})
	time.Sleep(2 * time.Millisecond)
	tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})
	tr.Close()

	require.Len(t, store.rows, 1)
	require.Empty(t, store.rows[0].WorkflowID)
}

func TestSlowQueryTracer_ConWorkflow_PersisteElUUIDReal(t *testing.T) {
	store := &stubSlowStore{}
	tr := NewSlowQueryTracer(&testInnerTracer{}, store, nil, 1, 0)

	quiero := uuid.New()
	base := WithWorkflowID(context.Background(), quiero)
	ctx := tr.TraceQueryStart(base, nil, pgx.TraceQueryStartData{SQL: "SELECT 1"})
	time.Sleep(2 * time.Millisecond)
	tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{})
	tr.Close()

	require.Len(t, store.rows, 1)
	require.Equal(t, quiero.String(), store.rows[0].WorkflowID)
}

func TestFnLogger_SinWorkflow_NoPersisteElCentinela(t *testing.T) {
	store := &stubFnStore{}
	f := NewFnLogger(store, nil, 1)

	exit := f.EnterContext(context.Background(), "observation.Save", "observation", nil)
	exit(nil)
	f.Close()

	require.Len(t, store.rows, 1)
	require.Empty(t, store.rows[0].WorkflowID)
}

func TestFnLogger_ConWorkflow_PersisteElUUIDReal(t *testing.T) {
	store := &stubFnStore{}
	f := NewFnLogger(store, nil, 1)

	quiero := uuid.New()
	exit := f.EnterContext(WithWorkflowID(context.Background(), quiero), "observation.Save", "observation", nil)
	exit(nil)
	f.Close()

	require.Len(t, store.rows, 1)
	require.Equal(t, quiero.String(), store.rows[0].WorkflowID)
}

func TestErrorTracker_SinWorkflow_NoPersisteElCentinela(t *testing.T) {
	store := &fakeErrStore{}
	tr := NewErrorTracker(store, nil)

	tr.Record(context.Background(), errors.New("boom"), "test")
	tr.Close()

	require.Equal(t, 1, store.count())
	require.Empty(t, store.last().WorkflowID)
}

func TestErrorTracker_ConWorkflow_PersisteElUUIDReal(t *testing.T) {
	store := &fakeErrStore{}
	tr := NewErrorTracker(store, nil)

	quiero := uuid.New()
	tr.Record(WithWorkflowID(context.Background(), quiero), errors.New("boom"), "test")
	tr.Close()

	require.Equal(t, 1, store.count())
	require.Equal(t, quiero.String(), store.last().WorkflowID)
}

// El motivo por el que los 4 sitios estaban mal es que .String() sobre el Nil
// produce algo que PARECE un uuid. Este test fija esa premisa: si alguna vez
// uuid.Nil.String() devolviera "", el guard sería redundante y este test lo diría.
func TestWorkflowIDForRow_ElNilNoEsCadenaVacia(t *testing.T) {
	require.NotEmpty(t, uuid.Nil.String(),
		"si esto cambia, el NULLIF del INSERT bastaría solo y workflowIDForRow sobra")
	require.Empty(t, workflowIDForRow(uuid.Nil))

	real := uuid.New()
	require.Equal(t, real.String(), workflowIDForRow(real))
}

// nullableUUIDPtr colapsa los DOS no-valores al mismo NULL. El segundo caso (un
// puntero a uuid.Nil) es el que dejaría entrar el centinela por el camino de
// escritura de workflows si un caller armara la fila con un "vacío" mal tipado.
func TestNullableUUIDPtr_ColapsaNilYPunteroAlNil(t *testing.T) {
	require.Nil(t, nullableUUIDPtr(nil))

	cero := uuid.Nil
	require.Nil(t, nullableUUIDPtr(&cero),
		"un puntero a uuid.Nil es tan poco un actor como un puntero nil")

	real := uuid.New()
	require.Equal(t, real, nullableUUIDPtr(&real))
}
