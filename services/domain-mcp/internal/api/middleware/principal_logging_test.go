package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/auth/apikey"
)

// DOMAINSERV-81: había DOS familias de context keys. PrincipalCtx escribía user_id
// y org_id en internal/api/ctxkeys, pero el ctxEnrichHandler los lee de
// internal/logging. Claves distintas de paquetes distintos nunca coinciden, así que
// los logs salían sin user_id ni organization_id aunque el principal estuviera
// resuelto. Este test cubre el puente.
//
// QUÉ log se verifica y por qué: el de los HANDLERS, no el "http request" de
// RequestLog. El context de Go viaja hacia adentro, nunca hacia afuera, así que
// RequestLog —que envuelve a PrincipalCtx— logea con su propio ctx y no puede ver
// lo que enriqueció un middleware interior. Su access log lleva request_id, que
// alcanza para correlacionar: filtrando por ese id se encuentran tanto el acceso
// como los logs del handler, y esos sí traen la identidad completa.

// logDelHandler devuelve la entrada que emite un handler ubicado al final de la
// cadena real (RequestLog → PrincipalCtx → handler).
func logDelHandler(t *testing.T, principal *apikey.Principal) map[string]any {
	t.Helper()
	logger, buf := capturaLogs()

	handler := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		logger.InfoContext(r.Context(), "trabajo del handler")
	})
	h := RequestLog(logger)(PrincipalCtx(handler))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)
	if principal != nil {
		req = req.WithContext(apikey.WithPrincipal(req.Context(), principal))
	}
	h.ServeHTTP(httptest.NewRecorder(), req)

	// el buffer trae 2 líneas: la del handler y el "http request" de RequestLog
	primera := bytes.SplitN(bytes.TrimSpace(buf.Bytes()), []byte("\n"), 2)[0]
	var entry map[string]any
	require.NoError(t, json.Unmarshal(primera, &entry))
	require.Equal(t, "trabajo del handler", entry["msg"], "se verifica el log del handler, no el access log")
	return entry
}

func TestPrincipalCtx_ServeHTTP_ConPrincipal_LogeaUserIDYOrganizationID(t *testing.T) {
	orgID, userID := uuid.New(), uuid.New()

	entry := logDelHandler(t, &apikey.Principal{
		UserID:         userID.String(),
		OrganizationID: orgID.String(),
	})

	require.Equal(t, userID.String(), entry["user_id"])
	require.Equal(t, orgID.String(), entry["organization_id"])
	require.NotEmpty(t, entry["request_id"], "los tres campos de correlación viajan juntos")
}

func TestPrincipalCtx_ServeHTTP_SinPrincipal_NoInventaCamposDeCorrelacion(t *testing.T) {
	entry := logDelHandler(t, nil)

	require.NotContains(t, entry, "user_id", "sin principal no hay user_id que logear")
	require.NotContains(t, entry, "organization_id")
	require.NotEmpty(t, entry["request_id"], "el request_id no depende del principal: un 401 también se correlaciona")
}
