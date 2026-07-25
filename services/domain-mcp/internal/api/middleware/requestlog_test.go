package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/logging"
)

// DOMANSERV-81 criterio 2: la correlación log↔métrica se hace por request_id, y
// para que el ctxEnrichHandler pueda inyectarlo alguien tiene que ponerlo en el
// context. Antes NADIE lo hacía: WithRequestID no tenía llamadores en producción,
// así que los logs salían sin un solo campo de correlación.

// capturaLogs devuelve un logger que escribe JSON a un buffer, con el mismo
// ctxEnrichHandler que usa producción. Sin eso el test pasaría verde con el
// request_id en el context pero sin llegar nunca al log.
func capturaLogs() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return logging.Setup(logging.Config{Format: "json", Level: "info", Writer: &buf}), &buf
}

func TestRequestLog_ServeHTTP_SinHeaderEntrante_GeneraRequestIDYLoLogea(t *testing.T) {
	logger, buf := capturaLogs()

	var visto string
	h := RequestLog(logger)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		visto = logging.RequestIDFromContext(r.Context())
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/x", nil))

	require.NotEmpty(t, visto, "el handler downstream debe ver un request_id en el context")
	_, err := uuid.Parse(visto)
	require.NoError(t, err, "el request_id generado debe ser un UUID")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry))
	require.Equal(t, visto, entry["request_id"], "el log debe llevar el mismo request_id que vio el handler")
}

func TestRequestLog_ServeHTTP_ConXRequestIDEntrante_PropagaElRecibido(t *testing.T) {
	logger, buf := capturaLogs()

	entrante := "trace-de-otro-servicio-123"
	var visto string
	h := RequestLog(logger)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		visto = logging.RequestIDFromContext(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)
	req.Header.Set("X-Request-Id", entrante)
	h.ServeHTTP(httptest.NewRecorder(), req)

	require.Equal(t, entrante, visto, "un request_id entrante se propaga, no se descarta")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry))
	require.Equal(t, entrante, entry["request_id"])
}

func TestRequestLog_ServeHTTP_Siempre_DevuelveElRequestIDEnLaRespuesta(t *testing.T) {
	logger, _ := capturaLogs()

	h := RequestLog(logger)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/x", nil))

	require.NotEmpty(t, rec.Header().Get("X-Request-Id"),
		"el cliente necesita el request_id para poder correlacionar del otro lado")
}

// Sabotaje: si el middleware dejara de inyectar en el context, el log quedaría sin
// request_id. Este test verifica que el propio arnés lo detecta.
func TestRequestLog_Sabotaje_SinInyeccionEnContexto_ElLogQuedaSinRequestID(t *testing.T) {
	logger, buf := capturaLogs()

	logger.InfoContext(context.Background(), "sin request id en el ctx")

	var entry map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &entry))
	_, tiene := entry["request_id"]
	require.False(t, tiene, "sin inyección previa el campo NO debe aparecer — si aparece, el arnés no prueba nada")
}
