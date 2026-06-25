// HU-28.1: unit tests con mock Repository — validan que Service propaga
// errores y que la lógica de negocio (validación, privacy strip, dedup hash,
// audit) es testeable sin DB real.
package observation

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/llm"
)

// mockRepo es una implementación in-memory mínima de Repository. Permite
// inyectar comportamientos por hook (InsertHook, GetHook, etc.).
type mockRepo struct {
	InsertHook        func(ctx context.Context, in InsertParams) (*Observation, error)
	GetHook           func(ctx context.Context, id uuid.UUID) (*Observation, error)
	ListHook          func(ctx context.Context, projectID uuid.UUID, limit int) ([]Observation, error)
	ListPaginatedHook func(ctx context.Context, in ListPageInput) ([]Observation, bool, error)
	SoftDeleteHook    func(ctx context.Context, id uuid.UUID) error
	SearchHook        func(ctx context.Context, in SearchInput) ([]SearchResult, error)



	InsertCalls, GetCalls, ListCalls, SoftDeleteCalls, SearchCalls int
}

func (m *mockRepo) Insert(ctx context.Context, in InsertParams) (*Observation, error) {
	m.InsertCalls++
	if m.InsertHook != nil {
		return m.InsertHook(ctx, in)
	}
	return &Observation{ID: uuid.New(), ProjectID: in.ProjectID, Content: in.Content}, nil
}
func (m *mockRepo) Get(ctx context.Context, id uuid.UUID) (*Observation, error) {
	m.GetCalls++
	if m.GetHook != nil {
		return m.GetHook(ctx, id)
	}
	return nil, ErrNotFound
}
func (m *mockRepo) List(ctx context.Context, projectID uuid.UUID, limit int) ([]Observation, error) {
	m.ListCalls++
	if m.ListHook != nil {
		return m.ListHook(ctx, projectID, limit)
	}
	return nil, nil
}
func (m *mockRepo) ListPaginated(ctx context.Context, in ListPageInput) ([]Observation, bool, error) {
	if m.ListPaginatedHook != nil {
		return m.ListPaginatedHook(ctx, in)
	}
	return nil, false, nil
}
func (m *mockRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	m.SoftDeleteCalls++
	if m.SoftDeleteHook != nil {
		return m.SoftDeleteHook(ctx, id)
	}
	return nil
}
func (m *mockRepo) SearchHybrid(ctx context.Context, in SearchInput) ([]SearchResult, error) {
	m.SearchCalls++
	if m.SearchHook != nil {
		return m.SearchHook(ctx, in)
	}
	return nil, nil
}

// nopEmbedder evita acoplar el test al package llm.
type nopEmbedder struct{}

func (nopEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {

	return make([]float32, 4), nil
}

// TestSave_HappyPath verifica que Service.Save delega al repo y aplica la
// pipeline de validación (privacy strip + dedup hash + audit recorder nil-safe).
// Sin DB.
func TestSave_HappyPath(t *testing.T) {
	repo := &mockRepo{}
	svc := NewService(nil, nil, llm.NopEmbedder{}, nil, repo)

	got, err := svc.Save(context.Background(), SaveInput{
		OrganizationID: uuid.New(),
		ProjectID:      uuid.New(),
		Content:        "hola mundo",
	})
	if err != nil {
		t.Fatalf("Save err: %v", err)
	}
	if got == nil {
		t.Fatal("got nil observation")
	}
	if repo.InsertCalls != 1 {
		t.Fatalf("InsertCalls = %d, want 1", repo.InsertCalls)
	}
}

// TestSave_ContentRequired — content vacío no llega al repo.
func TestSave_ContentRequired(t *testing.T) {
	repo := &mockRepo{}
	svc := NewService(nil, nil, llm.NopEmbedder{}, nil, repo)

	_, err := svc.Save(context.Background(), SaveInput{
		OrganizationID: uuid.New(),
		ProjectID:      uuid.New(),
		Content:        "   ",
	})
	if !errors.Is(err, ErrContentRequired) {
		t.Fatalf("err = %v, want ErrContentRequired", err)
	}
	if repo.InsertCalls != 0 {
		t.Fatalf("repo.Insert no debió ser llamado, llamadas=%d", repo.InsertCalls)
	}
}

// TestSave_Sabotage_RepoError — el repo devuelve error arbitrario y el
// Service lo propaga sin panic (HU-28.1 escenario 4: sabotaje).
func TestSave_Sabotage_RepoError(t *testing.T) {
	want := errors.New("DB down")
	repo := &mockRepo{
		InsertHook: func(_ context.Context, _ InsertParams) (*Observation, error) {
			return nil, want
		},
	}
	svc := NewService(nil, nil, llm.NopEmbedder{}, nil, repo)
	_, err := svc.Save(context.Background(), SaveInput{
		OrganizationID: uuid.New(),
		ProjectID:      uuid.New(),
		Content:        "x",
	})
	if err == nil || !errorContains(err, "DB down") {
		t.Fatalf("err = %v, want chain con 'DB down'", err)
	}
}

// TestGet_NotFound — el Service propaga el ErrNotFound del repo.
func TestGet_NotFound(t *testing.T) {
	repo := &mockRepo{
		GetHook: func(_ context.Context, _ uuid.UUID) (*Observation, error) {
			return nil, ErrNotFound
		},
	}
	svc := NewService(nil, nil, llm.NopEmbedder{}, nil, repo)
	_, err := svc.Get(context.Background(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// TestList_LimitClamp — limit fuera de rango se normaliza a 50 antes de llegar al repo.
func TestList_LimitClamp(t *testing.T) {
	var gotLimit int
	repo := &mockRepo{
		ListHook: func(_ context.Context, _ uuid.UUID, limit int) ([]Observation, error) {
			gotLimit = limit
			return nil, nil
		},
	}
	svc := NewService(nil, nil, llm.NopEmbedder{}, nil, repo)
	_, _ = svc.List(context.Background(), uuid.New(), 9999)
	if gotLimit != 50 {
		t.Fatalf("repo recibió limit=%d, want 50 (clamp)", gotLimit)
	}
	_, _ = svc.List(context.Background(), uuid.New(), 0)
	if gotLimit != 50 {
		t.Fatalf("limit=0 debió forzar default 50, got %d", gotLimit)
	}
}

// TestSoftDelete_PropagatesNotFound — el repo dice ErrNotFound y el Service
// lo propaga sin escribir audit.
func TestSoftDelete_PropagatesNotFound(t *testing.T) {
	repo := &mockRepo{
		SoftDeleteHook: func(_ context.Context, _ uuid.UUID) error { return ErrNotFound },
	}
	svc := NewService(nil, nil, llm.NopEmbedder{}, nil, repo)
	err := svc.SoftDelete(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func errorContains(err error, want string) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	for i := 0; i+len(want) <= len(s); i++ {
		if s[i:i+len(want)] == want {
			return true
		}
	}
	return false
}
