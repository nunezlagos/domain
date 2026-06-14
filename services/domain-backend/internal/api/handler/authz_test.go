// HU-28.3 — tests de helpers cross-org en handler/api.go.
package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/api/ctxkeys"
)

func TestOrgIDFromCtx(t *testing.T) {
	a := &API{}
	want := uuid.New()
	ctx := ctxkeys.WithOrgID(context.Background(), want)
	if got := a.orgID(ctx); got != want {
		t.Errorf("orgID()=%s, want %s", got, want)
	}
}

func TestOrgIDFromCtx_VacioRetornaNil(t *testing.T) {
	a := &API{}
	if got := a.orgID(context.Background()); got != uuid.Nil {
		t.Errorf("orgID()=%s, want uuid.Nil", got)
	}
}

func TestAuthorizeOrg_MismaOrg_OK(t *testing.T) {
	a := &API{}
	orgID := uuid.New()
	ctx := ctxkeys.WithOrgID(context.Background(), orgID)
	if err := a.authorizeOrg(ctx, orgID); err != nil {
		t.Errorf("authorizeOrg con misma org devolvió error: %v", err)
	}
}

func TestAuthorizeOrg_CrossOrg_DevuelveErrCrossOrg(t *testing.T) {
	a := &API{}
	caller := uuid.New()
	resource := uuid.New()
	ctx := ctxkeys.WithOrgID(context.Background(), caller)
	err := a.authorizeOrg(ctx, resource)
	if !errors.Is(err, ErrCrossOrg) {
		t.Errorf("authorizeOrg cross-org devolvió %v, want ErrCrossOrg", err)
	}
}

func TestAuthorizeOrg_SinCtx_Bloquea(t *testing.T) {
	// Si el middleware nunca seteó OrgID en ctx, a.orgID() devuelve uuid.Nil.
	// authorizeOrg debe rechazar cualquier resource (defense-in-depth).
	a := &API{}
	resource := uuid.New()
	err := a.authorizeOrg(context.Background(), resource)
	if !errors.Is(err, ErrCrossOrg) {
		t.Errorf("authorizeOrg sin ctx devolvió %v, want ErrCrossOrg", err)
	}
}
