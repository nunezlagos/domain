// HU-02.2 RBAC unit tests.

package rbac

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/saargo/domain/internal/auth/apikey"
)

func TestIsBuiltin(t *testing.T) {
	for _, r := range []Role{RoleOwner, RoleAdmin, RoleMaintainer, RoleMember, RoleViewer} {
		require.True(t, IsBuiltin(r))
	}
	require.False(t, IsBuiltin("custom-auditor"))
	require.False(t, IsBuiltin(""))
}

func TestAtLeast(t *testing.T) {
	// owner >= todos
	for _, r := range []Role{RoleOwner, RoleAdmin, RoleMaintainer, RoleMember, RoleViewer} {
		require.True(t, AtLeast(RoleOwner, r), "owner should satisfy %s", r)
	}
	// viewer no satisface nada > viewer
	require.False(t, AtLeast(RoleViewer, RoleMember))
	require.False(t, AtLeast(RoleViewer, RoleAdmin))
	// member satisface viewer
	require.True(t, AtLeast(RoleMember, RoleViewer))
	// custom role NUNCA satisface jerarquía built-in
	require.False(t, AtLeast("custom", RoleViewer))
}

func TestChecker_Builtin_HappyPath(t *testing.T) {
	c := &Checker{}
	require.NoError(t, c.Check(context.Background(), "org-1", RoleMember, ResObservation, ActWrite))
	require.NoError(t, c.Check(context.Background(), "org-1", RoleViewer, ResObservation, ActRead))
	require.NoError(t, c.Check(context.Background(), "org-1", RoleAdmin, ResMember, ActAdmin))
	require.NoError(t, c.Check(context.Background(), "org-1", RoleOwner, ResBilling, ActAdmin))
}

func TestChecker_Builtin_Denied(t *testing.T) {
	c := &Checker{}
	// viewer NO puede write
	require.ErrorIs(t, c.Check(context.Background(), "org-1", RoleViewer, ResObservation, ActWrite), ErrForbidden)
	// member NO puede admin de members
	require.ErrorIs(t, c.Check(context.Background(), "org-1", RoleMember, ResMember, ActAdmin), ErrForbidden)
	// admin NO puede touch billing (solo owner)
	require.ErrorIs(t, c.Check(context.Background(), "org-1", RoleAdmin, ResBilling, ActWrite), ErrForbidden)
	// maintainer NO puede manage members
	require.ErrorIs(t, c.Check(context.Background(), "org-1", RoleMaintainer, ResMember, ActWrite), ErrForbidden)
}

func TestChecker_Custom_NoResolver_Forbidden(t *testing.T) {
	c := &Checker{}
	err := c.Check(context.Background(), "org-1", "custom-auditor", ResAuditLog, ActRead)
	require.ErrorIs(t, err, ErrForbidden)
}

// fakeCustomResolver para tests.
type fakeCustomResolver struct {
	allow bool
	err   error
}

func (f *fakeCustomResolver) HasPermission(_ context.Context, _, _ string, _ Resource, _ Action) (bool, error) {
	return f.allow, f.err
}

func TestChecker_Custom_WithResolver(t *testing.T) {
	c := &Checker{CustomResolver: &fakeCustomResolver{allow: true}}
	require.NoError(t, c.Check(context.Background(), "org-1", "custom-auditor", ResAuditLog, ActRead))

	c.CustomResolver = &fakeCustomResolver{allow: false}
	require.ErrorIs(t, c.Check(context.Background(), "org-1", "custom-auditor", ResAuditLog, ActWrite), ErrForbidden)
}

// fakeContext stub principal en context.
func ctxWithPrincipal(p *apikey.Principal) context.Context {
	r := httptest.NewRequest("GET", "/x", nil)
	// usar el middleware para inyectar
	var done http.Handler = http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {})
	// truco: usar wrap manual con contexto
	_ = done
	// Mejor: usar mismo helper que el middleware
	// Pero apikey.FromContext usa privateKey unexported; necesitamos crear via middleware.
	res := &fakeResolver{principal: p}
	m := &apikey.Middleware{Resolver: res}
	// crear pt válido stub
	pt, _, _ := apikey.GeneratePlaintext("dev")
	r.Header.Set("Authorization", "Bearer "+pt)
	res.expected = pt
	rec := httptest.NewRecorder()
	var captured context.Context
	m.Wrap(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		captured = req.Context()
	})).ServeHTTP(rec, r)
	return captured
}

type fakeResolver struct {
	expected  string
	principal *apikey.Principal
}

func (f *fakeResolver) Resolve(_ context.Context, plaintext string) (*apikey.Principal, error) {
	if plaintext == f.expected && f.principal != nil {
		return f.principal, nil
	}
	return nil, apikey.ErrUnauthorized
}

func TestRequireRole_Allows(t *testing.T) {
	p := &apikey.Principal{UserID: "u1", OrganizationID: "o1", Role: string(RoleAdmin)}
	ctx := ctxWithPrincipal(p)
	require.NotNil(t, ctx)

	called := false
	h := RequireRole(RoleMaintainer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil).WithContext(ctx)
	h.ServeHTTP(rec, req)
	require.True(t, called)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestRequireRole_Denies(t *testing.T) {
	p := &apikey.Principal{UserID: "u1", OrganizationID: "o1", Role: string(RoleViewer)}
	ctx := ctxWithPrincipal(p)

	called := false
	h := RequireRole(RoleAdmin)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil).WithContext(ctx)
	h.ServeHTTP(rec, req)
	require.False(t, called)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequireRole_NoPrincipal_Forbidden(t *testing.T) {
	h := RequireRole(RoleViewer)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil) // no principal in context
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

func TestRequirePermission_Allows(t *testing.T) {
	p := &apikey.Principal{UserID: "u1", OrganizationID: "o1", Role: string(RoleMember)}
	ctx := ctxWithPrincipal(p)
	checker := &Checker{}

	h := RequirePermission(checker, ResObservation, ActWrite)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil).WithContext(ctx)
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestRequirePermission_Denies(t *testing.T) {
	p := &apikey.Principal{UserID: "u1", OrganizationID: "o1", Role: string(RoleViewer)}
	ctx := ctxWithPrincipal(p)
	checker := &Checker{}

	h := RequirePermission(checker, ResObservation, ActWrite)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil).WithContext(ctx)
	h.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)
}

// Sabotaje: matrix coverage — cada built-in role tiene al menos 1 permission.
func TestSabotage_Matrix_AllBuiltinRolesHavePermissions(t *testing.T) {
	for _, role := range []Role{RoleOwner, RoleAdmin, RoleMaintainer, RoleMember, RoleViewer} {
		require.NotEmpty(t, matrix[role], "role %s must have permissions in matrix", role)
	}
}

// Sabotaje: viewer NO puede write/delete/admin de ningún resource.
func TestSabotage_Viewer_OnlyRead(t *testing.T) {
	for res, actions := range matrix[RoleViewer] {
		for _, act := range actions {
			require.Equal(t, ActRead, act, "viewer NO debe tener %s sobre %s", act, res)
		}
	}
}

// Sabotaje: viewer is read-only — write/delete/admin always forbidden.
func TestSabotage_Viewer_ForbiddenForMutations(t *testing.T) {
	c := &Checker{}
	for _, res := range []Resource{ResObservation, ResProject, ResAgent, ResFlow, ResSkill} {
		for _, act := range []Action{ActWrite, ActDelete, ActAdmin} {
			err := c.Check(context.Background(), "o", RoleViewer, res, act)
			require.ErrorIsf(t, err, ErrForbidden, "viewer NO debe poder %s sobre %s", act, res)
		}
	}
}
