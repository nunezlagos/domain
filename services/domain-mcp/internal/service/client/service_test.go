// REQ-28.1: unit tests del Service sin DB real. Validan que las reglas de
// negocio (validaciones de input, mapping de errores, propagación al repo)
// son correctas y que audit nil-safe no rompe el flujo.
package client

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

func newSvc(repo Repository) *Service {
	return NewService(nil, nil, repo)
}



func TestCreate_HappyPath(t *testing.T) {
	repo := &mockRepo{}
	svc := newSvc(repo)

	orgID := uuid.New()
	c, err := svc.Create(context.Background(), orgID, CreateInput{
		Name: "Acme Corp",
		Slug: "acme-corp",
	})
	if err != nil {
		t.Fatalf("Create err: %v", err)
	}
	if c == nil {
		t.Fatal("nil client")
	}
	if repo.InsertCalls != 1 {
		t.Fatalf("InsertCalls=%d want 1", repo.InsertCalls)
	}
	if c.Status != StatusActive {
		t.Fatalf("default status=%q want %q", c.Status, StatusActive)
	}
}

func TestCreate_NameTooShort(t *testing.T) {
	repo := &mockRepo{}
	svc := newSvc(repo)
	_, err := svc.Create(context.Background(), uuid.New(), CreateInput{
		Name: "A",
		Slug: "acme",
	})
	if !errors.Is(err, ErrInvalidName) {
		t.Fatalf("err=%v want ErrInvalidName", err)
	}
	if repo.InsertCalls != 0 {
		t.Fatalf("repo.Insert no debió llamarse, calls=%d", repo.InsertCalls)
	}
}

func TestCreate_InvalidSlug(t *testing.T) {


	cases := []string{"acme_corp", "-acme", "acme-", "acme corp", ""}
	for _, slug := range cases {
		repo := &mockRepo{}
		svc := newSvc(repo)
		_, err := svc.Create(context.Background(), uuid.New(), CreateInput{
			Name: "Acme",
			Slug: slug,
		})
		if !errors.Is(err, ErrInvalidSlug) {
			t.Fatalf("slug=%q err=%v want ErrInvalidSlug", slug, err)
		}
	}
}

func TestCreate_InvalidStatus(t *testing.T) {
	repo := &mockRepo{}
	svc := newSvc(repo)
	_, err := svc.Create(context.Background(), uuid.New(), CreateInput{
		Name:   "Acme",
		Slug:   "acme",
		Status: "weird",
	})
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("err=%v want ErrInvalidStatus", err)
	}
}

func TestCreate_InvalidEmail(t *testing.T) {
	repo := &mockRepo{}
	svc := newSvc(repo)
	_, err := svc.Create(context.Background(), uuid.New(), CreateInput{
		Name:         "Acme",
		Slug:         "acme",
		ContactEmail: "not-an-email",
	})
	if !errors.Is(err, ErrInvalidEmail) {
		t.Fatalf("err=%v want ErrInvalidEmail", err)
	}
}

func TestCreate_EmptyEmailAllowed(t *testing.T) {
	repo := &mockRepo{}
	svc := newSvc(repo)
	_, err := svc.Create(context.Background(), uuid.New(), CreateInput{
		Name:         "Acme",
		Slug:         "acme",
		ContactEmail: "",
	})
	if err != nil {
		t.Fatalf("empty email debió aceptarse, err=%v", err)
	}
}

func TestCreate_InvalidTaxID(t *testing.T) {
	repo := &mockRepo{}
	svc := newSvc(repo)
	_, err := svc.Create(context.Background(), uuid.New(), CreateInput{
		Name:  "Acme",
		Slug:  "acme",
		TaxID: "!!", // ni RUT válido ni fallback
	})
	if !errors.Is(err, ErrInvalidTaxID) {
		t.Fatalf("err=%v want ErrInvalidTaxID", err)
	}
}

func TestCreate_SlugConflict_MapsTo_ErrClientSlugExists(t *testing.T) {


	pgErr := &pgconn.PgError{Code: pgerrcode.UniqueViolation, ConstraintName: "clients_slug_key"}
	repo := &mockRepo{
		InsertHook: func(_ context.Context, _ InsertParams) (*Client, error) {
			return nil, pgErr
		},
	}
	svc := newSvc(repo)
	_, err := svc.Create(context.Background(), uuid.New(), CreateInput{
		Name: "Acme",
		Slug: "acme",
	})
	if !errors.Is(err, ErrClientSlugExists) {
		t.Fatalf("err=%v want ErrClientSlugExists", err)
	}
}



func TestGet_ByUUID_HitsGetByID(t *testing.T) {
	id := uuid.New()
	repo := &mockRepo{
		GetByIDHook: func(_ context.Context, _, gotID uuid.UUID) (*Client, error) {
			if gotID != id {
				t.Errorf("repo recibió id=%s want %s", gotID, id)
			}
			return &Client{ID: id}, nil
		},
	}
	svc := newSvc(repo)
	if _, err := svc.Get(context.Background(), uuid.New(), id.String()); err != nil {
		t.Fatalf("Get err: %v", err)
	}
	if repo.GetByIDCalls != 1 || repo.GetBySlugCalls != 0 {
		t.Fatalf("calls byID=%d bySlug=%d want 1/0", repo.GetByIDCalls, repo.GetBySlugCalls)
	}
}

func TestGet_BySlug_HitsGetBySlug(t *testing.T) {
	repo := &mockRepo{
		GetBySlugHook: func(_ context.Context, _ uuid.UUID, slug string) (*Client, error) {
			if slug != "acme-corp" {
				t.Errorf("slug=%q", slug)
			}
			return &Client{Slug: slug}, nil
		},
	}
	svc := newSvc(repo)
	if _, err := svc.Get(context.Background(), uuid.New(), "acme-corp"); err != nil {
		t.Fatalf("Get err: %v", err)
	}
	if repo.GetBySlugCalls != 1 || repo.GetByIDCalls != 0 {
		t.Fatalf("calls bySlug=%d byID=%d want 1/0", repo.GetBySlugCalls, repo.GetByIDCalls)
	}
}

func TestGet_NotFound_Propagates(t *testing.T) {
	repo := &mockRepo{
		GetBySlugHook: func(_ context.Context, _ uuid.UUID, _ string) (*Client, error) {
			return nil, ErrClientNotFound
		},
	}
	svc := newSvc(repo)
	_, err := svc.Get(context.Background(), uuid.New(), "nope")
	if !errors.Is(err, ErrClientNotFound) {
		t.Fatalf("err=%v want ErrClientNotFound", err)
	}
}



func TestList_LimitClampAndDefault(t *testing.T) {
	var gotLimit int
	repo := &mockRepo{
		ListHook: func(_ context.Context, _ uuid.UUID, f ListFilter) ([]*Client, int64, error) {
			gotLimit = f.Limit
			return nil, 0, nil
		},
	}
	svc := newSvc(repo)
	_, _, _ = svc.List(context.Background(), uuid.New(), ListFilter{Limit: 9999})
	if gotLimit != 50 {
		t.Fatalf("limit=%d want clamp 50", gotLimit)
	}
	_, _, _ = svc.List(context.Background(), uuid.New(), ListFilter{Limit: 0})
	if gotLimit != 50 {
		t.Fatalf("limit=0 debió ser 50, got %d", gotLimit)
	}
}

func TestList_InvalidStatus(t *testing.T) {
	repo := &mockRepo{}
	svc := newSvc(repo)
	_, _, err := svc.List(context.Background(), uuid.New(), ListFilter{Status: "weird"})
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("err=%v want ErrInvalidStatus", err)
	}
	if repo.ListCalls != 0 {
		t.Fatalf("repo.List no debió llamarse, calls=%d", repo.ListCalls)
	}
}



func TestUpdate_PatchOnlyName(t *testing.T) {
	id := uuid.New()
	orgID := uuid.New()
	prev := &Client{
		ID: id,
		Name: "old", Slug: "acme",
		Status:       StatusActive,
		ContactEmail: "x@y.com",
	}
	repo := &mockRepo{
		GetByIDHook: func(_ context.Context, _, _ uuid.UUID) (*Client, error) { return prev, nil },
		UpdateHook: func(_ context.Context, _, _ uuid.UUID, in UpdateParams) (*Client, error) {
			if in.Name != "new" {
				t.Errorf("name=%q want new", in.Name)
			}
			if in.ContactEmail != "x@y.com" {
				t.Errorf("email no debió cambiar, got %q", in.ContactEmail)
			}
			if in.Status != StatusActive {
				t.Errorf("status no debió cambiar, got %q", in.Status)
			}
			return &Client{ID: id, Name: in.Name, Status: in.Status}, nil
		},
	}
	svc := newSvc(repo)
	newName := "new"
	_, err := svc.Update(context.Background(), orgID, id, UpdateInput{Name: &newName})
	if err != nil {
		t.Fatalf("Update err: %v", err)
	}
}

func TestUpdate_NotFound_Propagates(t *testing.T) {
	repo := &mockRepo{
		GetByIDHook: func(_ context.Context, _, _ uuid.UUID) (*Client, error) {
			return nil, ErrClientNotFound
		},
	}
	svc := newSvc(repo)
	_, err := svc.Update(context.Background(), uuid.New(), uuid.New(), UpdateInput{})
	if !errors.Is(err, ErrClientNotFound) {
		t.Fatalf("err=%v want ErrClientNotFound", err)
	}
}

func TestUpdate_InvalidStatus(t *testing.T) {
	prev := &Client{Status: StatusActive}
	repo := &mockRepo{
		GetByIDHook: func(_ context.Context, _, _ uuid.UUID) (*Client, error) { return prev, nil },
	}
	svc := newSvc(repo)
	bad := "bogus"
	_, err := svc.Update(context.Background(), uuid.New(), uuid.New(), UpdateInput{Status: &bad})
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("err=%v want ErrInvalidStatus", err)
	}
	if repo.UpdateCalls != 0 {
		t.Fatalf("repo.Update no debió llamarse, calls=%d", repo.UpdateCalls)
	}
}



func TestDelete_PropagatesNotFound(t *testing.T) {
	repo := &mockRepo{
		SoftDeleteHook: func(_ context.Context, _, _ uuid.UUID) error { return ErrClientNotFound },
	}
	svc := newSvc(repo)
	err := svc.Delete(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrClientNotFound) {
		t.Fatalf("err=%v want ErrClientNotFound", err)
	}
}

func TestRestore_OK(t *testing.T) {
	repo := &mockRepo{}
	svc := newSvc(repo)
	if err := svc.Restore(context.Background(), uuid.New(), uuid.New()); err != nil {
		t.Fatalf("Restore err: %v", err)
	}
	if repo.RestoreCalls != 1 {
		t.Fatalf("RestoreCalls=%d want 1", repo.RestoreCalls)
	}
}



func TestSetStatus_InvalidEnum(t *testing.T) {
	repo := &mockRepo{}
	svc := newSvc(repo)
	_, err := svc.SetStatus(context.Background(), uuid.New(), uuid.New(), "bogus")
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("err=%v want ErrInvalidStatus", err)
	}
	if repo.SetStatusCalls != 0 || repo.GetByIDCalls != 0 {
		t.Fatalf("repo no debió llamarse")
	}
}

func TestSetStatus_HappyPath(t *testing.T) {
	id := uuid.New()
	orgID := uuid.New()
	repo := &mockRepo{
		GetByIDHook: func(_ context.Context, _, _ uuid.UUID) (*Client, error) {
			return &Client{ID: id, Status: StatusActive}, nil
		},
		SetStatusHook: func(_ context.Context, _, _ uuid.UUID, st string) (*Client, error) {
			return &Client{ID: id, Status: st}, nil
		},
	}
	svc := newSvc(repo)
	c, err := svc.SetStatus(context.Background(), orgID, id, StatusInactive)
	if err != nil {
		t.Fatalf("SetStatus err: %v", err)
	}
	if c.Status != StatusInactive {
		t.Fatalf("status=%q want %q", c.Status, StatusInactive)
	}
}



func TestCreate_Sabotage_RepoError(t *testing.T) {
	want := errors.New("db down")
	repo := &mockRepo{
		InsertHook: func(_ context.Context, _ InsertParams) (*Client, error) { return nil, want },
	}
	svc := newSvc(repo)
	_, err := svc.Create(context.Background(), uuid.New(), CreateInput{
		Name: "Acme", Slug: "acme",
	})
	if err == nil || !errors.Is(err, want) {
		t.Fatalf("err=%v want chain con want", err)
	}
}
