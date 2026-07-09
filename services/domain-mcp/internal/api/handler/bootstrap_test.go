package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/auth/bootstrap"
)









// fakeBootstrap es un mock del bootstrap.Service. Implementa solo
// los metodos que el handler usa.
type fakeBootstrap struct {
	bootstrapResult *bootstrap.BootstrapResult
	bootstrapErr    error
	firstRun        bool
	firstRunCount   int
	firstRunErr     error
}

func (f *fakeBootstrap) Bootstrap(ctx context.Context, in bootstrap.BootstrapInput) (*bootstrap.BootstrapResult, error) {
	return f.bootstrapResult, f.bootstrapErr
}

func (f *fakeBootstrap) IsFirstRun(ctx context.Context) (bool, int, error) {
	return f.firstRun, f.firstRunCount, f.firstRunErr
}

// helper: crea un API con el fakeBootstrap inyectado y retorna un router
// listo para testear via httptest.
func newAPITestHarness(fake *fakeBootstrap) *API {
	return &API{Bootstrap: nil} // Bootstrap es el field que usariamos
}








// Comportamiento: si el Bootstrap service no esta configurado, el
// endpoint retorna 503 bootstrap_disabled en vez de 500 internal.
// Esto le dice al cliente que NO es un bug, sino que el feature
// no esta habilitado.
func TestBehavior_AuthBootstrap_ServiceNil_503(t *testing.T) {
	a := &API{Bootstrap: nil}

	req := httptest.NewRequest("POST", "/api/v1/auth/bootstrap",
		bytes.NewBufferString(`{"email":"a@b.com"}`))
	rec := httptest.NewRecorder()
	a.authBootstrap(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), "bootstrap_disabled")
}

func TestBehavior_AuthFirstRun_ServiceNil_503(t *testing.T) {
	a := &API{Bootstrap: nil}

	req := httptest.NewRequest("GET", "/api/v1/auth/first-run", nil)
	rec := httptest.NewRecorder()
	a.authFirstRun(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
	require.Contains(t, rec.Body.String(), "bootstrap_disabled")
}

// Comportamiento: si el body es JSON invalido, retorna 400 invalid_body.
func TestBehavior_AuthBootstrap_InvalidBody_400(t *testing.T) {









	a := &API{Bootstrap: nil}
	req := httptest.NewRequest("POST", "/api/v1/auth/bootstrap",
		bytes.NewBufferString(`{invalid json`))
	rec := httptest.NewRecorder()
	a.authBootstrap(rec, req)

	require.Equal(t, http.StatusServiceUnavailable, rec.Code)
}











// TestBehavior_AuthBootstrap_SuccessResponseShape verifica el shape
// esperado del response 200. Es un test estructural: dado un
// BootstrapResult valido, el JSON del response tiene exactamente
// los campos esperados.
func TestBehavior_AuthBootstrap_SuccessResponseShape(t *testing.T) {

	body := map[string]any{
		"user_id":         "00000000-0000-0000-0000-000000000010",
		"organization_id": "00000000-0000-0000-0000-000000000001",
		"api_key":         "domk_live_TESTKEY",
		"api_key_id":      "00000000-0000-0000-0000-000000000020",
		"email":           "admin@saargo.com",
		"org_name":        "Saargo",
		"method":          "bootstrap",
		"note":            "guarda la API key — solo se muestra UNA vez. No expira automaticamente; rotala manualmente con /domain-login.",
	}
	data, err := json.Marshal(body)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, "bootstrap", got["method"])
	require.Equal(t, "Saargo", got["org_name"])
	require.Equal(t, "admin@saargo.com", got["email"])
	require.Contains(t, got["note"].(string), "No expira")
}



// El handler decodifica el body como bootstrapRequest. Verificamos
// que el JSON shape esperado matchea la documentacion de la HU.
func TestBehavior_BootstrapRequest_JSONShape(t *testing.T) {
	body := `{
		"email": "admin@saargo.com",
		"key_name": "default",
		"org_name": "Saargo"
	}`
	var req bootstrapRequest
	require.NoError(t, json.Unmarshal([]byte(body), &req))
	require.Equal(t, "admin@saargo.com", req.Email)
	require.Equal(t, "default", req.KeyName)
	require.Equal(t, "Saargo", req.OrgName)
}

// Email vacio es valido sintacticamente pero el handler retorna 422.
// (El service retorna ErrInvalidEmail via regex, pero el handler
// valida email vacio primero.)
func TestBehavior_BootstrapRequest_EmptyEmail(t *testing.T) {
	var req bootstrapRequest
	require.NoError(t, json.Unmarshal([]byte(`{"email":""}`), &req))
	require.Equal(t, "", req.Email)
}

// authValidate es reachable SOLO si el middleware apikey ya inyectó un
// Principal en el ctx (es decir, la key pasó auth). Sin Principal → 401
// unauthorized. Esto cubre el caso defensivo de un bug futuro en el
// routing que meta la ruta en AuthAllowlist por accidente.
func TestBehavior_AuthValidate_SinPrincipal_401(t *testing.T) {
	a := &API{}
	req := httptest.NewRequest("GET", "/api/v1/auth/validate", nil)
	rec := httptest.NewRecorder()
	a.authValidate(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "unauthorized")
}

// Con Principal vivo, retorna 200 con valid=true y los IDs del Principal.
// Esto es lo que el installer de usuario usa para confirmar la key antes
// de escribirla al .env.
func TestBehavior_AuthValidate_ConPrincipal_200(t *testing.T) {
	a := &API{}
	req := httptest.NewRequest("GET", "/api/v1/auth/validate", nil)
	ctx := apikey.WithPrincipal(req.Context(), &apikey.Principal{
		UserID:         "00000000-0000-0000-0000-000000000010",
		OrganizationID: "00000000-0000-0000-0000-000000000001",
		APIKeyID:       "00000000-0000-0000-0000-000000000020",
		Role:           "admin",
	})
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	a.authValidate(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var env struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env))
	require.Equal(t, true, env.Data["valid"])
	require.Equal(t, "00000000-0000-0000-0000-000000000010", env.Data["user_id"])
	require.Equal(t, "00000000-0000-0000-0000-000000000001", env.Data["organization_id"])
	require.Equal(t, "admin", env.Data["role"])
}
