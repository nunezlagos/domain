// issue-02.2 RBAC unit tests.

package rbac

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"nunezlagos/domain/internal/auth/apikey"
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
	require.NoError(t, c.Check(context.Background(), RoleMember, ResObservation, ActWrite))
	require.NoError(t, c.Check(context.Background(), RoleViewer, ResObservation, ActRead))
	require.NoError(t, c.Check(context.Background(), RoleAdmin, ResMember, ActAdmin))
	require.NoError(t, c.Check(context.Background(), RoleOwner, ResBilling, ActAdmin))
}

func TestChecker_Builtin_Denied(t *testing.T) {
	c := &Checker{}
	// viewer NO puede write
	require.ErrorIs(t, c.Check(context.Background(), RoleViewer, ResObservation, ActWrite), ErrForbidden)
	// member NO puede admin de members
	require.ErrorIs(t, c.Check(context.Background(), RoleMember, ResMember, ActAdmin), ErrForbidden)
	// role inexistente
	require.ErrorIs(t, c.Check(context.Background(), "no-such-role", ResObservation, ActRead), ErrForbidden)
	// resource sin permiso
	require.ErrorIs(t, c.Check(context.Background(), RoleViewer, ResSecret, ActRead), ErrForbidden)
	// maintainer NO puede admin de skills
	require.ErrorIs(t, c.Check(context.Background(), RoleMaintainer, ResMember, ActWrite), ErrForbidden)
}

func TestChecker_Custom_NoResolver_Forbidden(t *testing.T) {
	c := &Checker{}
	err := c.Check(context.Background(), "custom-auditor", ResAuditLog, ActRead)
	require.ErrorIs(t, err, ErrForbidden)
}

// fakeContext stub principal en context.
func ctxWithPrincipal(p *apikey.Principal) context.Context {
	r := httptest.NewRequest("GET", "/x", nil)
	return apikey.WithPrincipal(r.Context(), p)
}

func TestRequireRole(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := RequireRole(RoleMember)(next)

	cases := []struct {
		name        string
		role        string
		wantStatus  int
	}{
		{"admin passes admin gate", "admin", http.StatusOK},
		{"owner passes admin gate", "owner", http.StatusOK},
		{"member passes member gate", "member", http.StatusOK},
		{"viewer fails member gate", "viewer", http.StatusForbidden},
		{"no principal", "", http.StatusForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/x", nil)
			if tc.role != "" {
				ctx := ctxWithPrincipal(&apikey.Principal{UserID: "u", OrganizationID: "org-1", Role: tc.role})
				r = r.WithContext(ctx)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			require.Equal(t, tc.wantStatus, w.Code)
		})
	}
}

func TestRequirePermission(t *testing.T) {
	checker := &Checker{}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := RequirePermission(checker, ResObservation, ActWrite)(next)

	// member puede write observation → 200
	r := httptest.NewRequest("GET", "/x", nil)
	r = r.WithContext(ctxWithPrincipal(&apikey.Principal{UserID: "u", OrganizationID: "org-1", Role: "member"}))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusOK, w.Code)

	// viewer NO puede write observation → 403
	r = httptest.NewRequest("GET", "/x", nil)
	r = r.WithContext(ctxWithPrincipal(&apikey.Principal{UserID: "u", OrganizationID: "org-1", Role: "viewer"}))
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	require.Equal(t, http.StatusForbidden, w.Code)
}
