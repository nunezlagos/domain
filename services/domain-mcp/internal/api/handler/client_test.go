// Tests del handler de clients (mandantes). Cubren validacion de body,
// gating por principal/orgID en ctx, y resolucion id_or_slug. Los paths
// que requieren DB (Create/Update reales) viven en client_integration_test.go.
package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/api/ctxkeys"
)

func TestCreateClient_SinAuth_Devuelve401(t *testing.T) {
	a := &API{}
	body := strings.NewReader(`{"name":"Acme","slug":"acme"}`)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/clients", body)
	w := httptest.NewRecorder()
	a.createClient(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", w.Code)
	}
}

func TestCreateClient_BodyInvalido_Devuelve400(t *testing.T) {
	a := &API{}
	ctx := ctxkeys.WithOrgID(context.Background(), uuid.New())
	r := httptest.NewRequest(http.MethodPost, "/api/v1/clients",
		strings.NewReader(`{not-json`)).WithContext(ctx)
	w := httptest.NewRecorder()
	a.createClient(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", w.Code)
	}
	if !strings.Contains(w.Body.String(), "invalid_body") {
		t.Errorf("expected error code invalid_body, body=%s", w.Body.String())
	}
}

func TestCreateClient_NameSlugVacio_Devuelve422(t *testing.T) {
	a := &API{}
	ctx := ctxkeys.WithOrgID(context.Background(), uuid.New())
	for _, tc := range []struct {
		name string
		body string
	}{
		{"sin name", `{"slug":"acme"}`},
		{"sin slug", `{"name":"Acme"}`},
		{"ambos vacios", `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/api/v1/clients",
				strings.NewReader(tc.body)).WithContext(ctx)
			w := httptest.NewRecorder()
			a.createClient(w, r)
			if w.Code != http.StatusUnprocessableEntity {
				t.Fatalf("status=%d want 422", w.Code)
			}
			if !strings.Contains(w.Body.String(), "validation_failed") {
				t.Errorf("expected validation_failed, body=%s", w.Body.String())
			}
		})
	}
}

func TestListClients_SinAuth_Devuelve401(t *testing.T) {
	a := &API{}
	r := httptest.NewRequest(http.MethodGet, "/api/v1/clients", nil)
	w := httptest.NewRecorder()
	a.listClients(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", w.Code)
	}
}

func TestUpdateClient_IDInvalido_Devuelve404(t *testing.T) {
	a := &API{}
	ctx := ctxkeys.WithOrgID(context.Background(), uuid.New())
	r := httptest.NewRequest(http.MethodPut, "/api/v1/clients/no-uuid",
		strings.NewReader(`{}`)).WithContext(ctx)
	r.SetPathValue("id", "no-uuid")
	w := httptest.NewRecorder()
	a.updateClient(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404", w.Code)
	}
}

func TestDeleteClient_SinAuth_Devuelve401(t *testing.T) {
	a := &API{}
	r := httptest.NewRequest(http.MethodDelete, "/api/v1/clients/"+uuid.NewString(), nil)
	r.SetPathValue("id", uuid.NewString())
	w := httptest.NewRecorder()
	a.deleteClient(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", w.Code)
	}
}

func TestRestoreClient_IDInvalido_Devuelve404(t *testing.T) {
	a := &API{}
	ctx := ctxkeys.WithOrgID(context.Background(), uuid.New())
	r := httptest.NewRequest(http.MethodPost, "/api/v1/clients/abc/restore", nil).WithContext(ctx)
	r.SetPathValue("id", "abc")
	w := httptest.NewRecorder()
	a.restoreClient(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d want 404", w.Code)
	}
}

func TestSetClientStatus_BodySinStatus_Devuelve422(t *testing.T) {
	a := &API{}
	ctx := ctxkeys.WithOrgID(context.Background(), uuid.New())
	id := uuid.NewString()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/clients/"+id+"/status",
		strings.NewReader(`{}`)).WithContext(ctx)
	r.SetPathValue("id", id)
	w := httptest.NewRecorder()
	a.setClientStatus(w, r)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d want 422 (status field requerido)", w.Code)
	}
}

func TestSetClientStatus_BodyInvalido_Devuelve400(t *testing.T) {
	a := &API{}
	ctx := ctxkeys.WithOrgID(context.Background(), uuid.New())
	id := uuid.NewString()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/clients/"+id+"/status",
		strings.NewReader(`{bad`)).WithContext(ctx)
	r.SetPathValue("id", id)
	w := httptest.NewRecorder()
	a.setClientStatus(w, r)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d want 400", w.Code)
	}
}

func TestGetClient_SinAuth_Devuelve401(t *testing.T) {
	a := &API{}
	r := httptest.NewRequest(http.MethodGet, "/api/v1/clients/acme", nil)
	r.SetPathValue("id_or_slug", "acme")
	w := httptest.NewRecorder()
	a.getClient(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want 401", w.Code)
	}
}

// Compile-time check: setClientStatusBody serializa el campo "status".
func TestSetClientStatusBody_RoundTrip(t *testing.T) {
	in := setClientStatusBody{Status: "active"}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !bytes.Contains(b, []byte(`"status":"active"`)) {
		t.Errorf("status field missing in JSON: %s", string(b))
	}
}
