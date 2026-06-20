// HU-28.3 — tests del middleware principal-ctx.
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/api/ctxkeys"
	"nunezlagos/domain/internal/auth/apikey"
)

// inspector handler captura el ctx para que el test pueda verificar
// los value objects inyectados por PrincipalCtx.
type inspector struct {
	gotOrgID  uuid.UUID
	gotUserID uuid.UUID
	called    bool
}

func (i *inspector) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	i.called = true
	i.gotOrgID = ctxkeys.OrgID(r.Context())
	i.gotUserID = ctxkeys.UserID(r.Context())
	w.WriteHeader(http.StatusOK)
}

func TestPrincipalCtx_InyectaOrgIDyUserID(t *testing.T) {
	orgID := uuid.New()
	userID := uuid.New()
	insp := &inspector{}
	h := PrincipalCtx(insp)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	ctx := apikey.WithPrincipal(req.Context(), &apikey.Principal{
		UserID:         userID.String(),
		OrganizationID: orgID.String(),
	})
	req = req.WithContext(ctx)

	h.ServeHTTP(httptest.NewRecorder(), req)

	if !insp.called {
		t.Fatal("downstream handler no fue invocado")
	}
	if insp.gotOrgID != orgID {
		t.Errorf("OrgID=%s, want %s", insp.gotOrgID, orgID)
	}
	if insp.gotUserID != userID {
		t.Errorf("UserID=%s, want %s", insp.gotUserID, userID)
	}
}

func TestPrincipalCtx_SinPrincipal_PasaSinTocarCtx(t *testing.T) {
	// Allowlist path o auth aún no corrió: PrincipalCtx no debe romper.
	insp := &inspector{}
	h := PrincipalCtx(insp)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)

	if !insp.called {
		t.Fatal("downstream handler no fue invocado")
	}
	if insp.gotOrgID != uuid.Nil {
		t.Errorf("OrgID=%s, want uuid.Nil", insp.gotOrgID)
	}
	if insp.gotUserID != uuid.Nil {
		t.Errorf("UserID=%s, want uuid.Nil", insp.gotUserID)
	}
}

func TestPrincipalCtx_UUIDInvalido_DejaNilSinCrashear(t *testing.T) {
	// Principal con UUIDs inválidos: el middleware no debe panic. OrgID
	// queda uuid.Nil y los handlers downstream deben rechazar via helper.
	insp := &inspector{}
	h := PrincipalCtx(insp)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	ctx := apikey.WithPrincipal(req.Context(), &apikey.Principal{
		UserID:         "not-a-uuid",
		OrganizationID: "also-not-a-uuid",
	})
	req = req.WithContext(ctx)

	h.ServeHTTP(httptest.NewRecorder(), req)

	if !insp.called {
		t.Fatal("downstream handler no fue invocado")
	}
	if insp.gotOrgID != uuid.Nil {
		t.Errorf("OrgID=%s, want uuid.Nil (parse falló)", insp.gotOrgID)
	}
}
