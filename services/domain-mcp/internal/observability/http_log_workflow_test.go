package observability

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// DOMAINSERV-189: el middleware inventaba un workflow ID en cada request cuando el
// cliente no mandaba X-Workflow-Id. Como el cliente MCP nunca lo manda, cada llamada
// suelta recibía el suyo y el Tracker le creaba una fila con total_tool_calls=1,
// project_id en ceros y sin cerrar. 30 "workflows" en 7 minutos, todos basura.
//
// Un workflow es una cadena de llamadas correlacionada a propósito, no un request.
// El id de un request individual ya lo da X-Request-Id, que no se toca.
func TestHTTPLogger_Middleware_SinHeader_NoInventaWorkflowID(t *testing.T) {
	store := &stubHTTPLogStore{}
	h := NewHTTPLogger(store, nil, 1)
	defer h.Close()

	var visto uuid.UUID
	mw := h.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		visto = WorkflowIDFromContext(r.Context())
	}))
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	require.Equal(t, uuid.Nil, visto,
		"una llamada suelta no puede recibir un workflow id: el Tracker le crearía una fila")
	require.Empty(t, rec.Header().Get("X-Workflow-Id"),
		"sin workflow no hay header que devolver — un UUID en la respuesta invita al cliente a reusarlo")
}

// El cliente que SÍ quiere correlacionar una cadena manda el header, y eso tiene que
// seguir funcionando: es la única forma de que la tabla reciba datos con sentido.
func TestHTTPLogger_Middleware_ConHeader_PropagaElWorkflowID(t *testing.T) {
	store := &stubHTTPLogStore{}
	h := NewHTTPLogger(store, nil, 1)
	defer h.Close()

	quiero := uuid.New()
	var visto uuid.UUID
	mw := h.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		visto = WorkflowIDFromContext(r.Context())
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Workflow-Id", quiero.String())
	mw.ServeHTTP(rec, req)

	require.Equal(t, quiero, visto, "el workflow id del cliente tiene que llegar al handler")
	require.Equal(t, quiero.String(), rec.Header().Get("X-Workflow-Id"))
}

// Un header corrupto no puede degradar a "invento uno": volvería el defecto por la
// puerta de atrás para cualquier cliente con un bug de formato.
func TestHTTPLogger_Middleware_HeaderInvalido_NoInventaWorkflowID(t *testing.T) {
	store := &stubHTTPLogStore{}
	h := NewHTTPLogger(store, nil, 1)
	defer h.Close()

	var visto uuid.UUID
	mw := h.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		visto = WorkflowIDFromContext(r.Context())
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("X-Workflow-Id", "no-soy-un-uuid")
	mw.ServeHTTP(rec, req)

	require.Equal(t, uuid.Nil, visto)
}

// El request id no comparte el destino del workflow id: sigue generándose siempre,
// porque correlacionar UN request es justamente para lo que sirve.
func TestHTTPLogger_Middleware_ElRequestIDSeSigueGenerando(t *testing.T) {
	store := &stubHTTPLogStore{}
	h := NewHTTPLogger(store, nil, 1)
	defer h.Close()

	mw := h.Middleware(nextHandler(&atomic.Int32{}))
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

	_, err := uuid.Parse(rec.Header().Get("X-Request-Id"))
	require.NoError(t, err, "quitar el workflow id no puede llevarse puesto el request id")
}
