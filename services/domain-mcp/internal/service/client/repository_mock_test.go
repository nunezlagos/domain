// REQ-28.1: mock Repository — permite unit-testear el Service sin DB real.
package client

import (
	"context"

	"github.com/google/uuid"
)

// mockRepo es una implementación in-memory mínima de Repository. Cada método
// expone un Hook para inyectar comportamientos por test, y contadores de
// invocación para verificar que el Service llamó al repo (y no algún path
// legacy).
type mockRepo struct {
	InsertHook     func(ctx context.Context, in InsertParams) (*Client, error)
	GetByIDHook    func(ctx context.Context, orgID, id uuid.UUID) (*Client, error)
	GetBySlugHook  func(ctx context.Context, orgID uuid.UUID, slug string) (*Client, error)
	ListHook       func(ctx context.Context, orgID uuid.UUID, f ListFilter) ([]*Client, int64, error)
	UpdateHook     func(ctx context.Context, orgID, id uuid.UUID, in UpdateParams) (*Client, error)
	SoftDeleteHook func(ctx context.Context, orgID, id uuid.UUID) error
	RestoreHook    func(ctx context.Context, orgID, id uuid.UUID) error
	SetStatusHook  func(ctx context.Context, orgID, id uuid.UUID, status string) (*Client, error)

	InsertCalls, GetByIDCalls, GetBySlugCalls, ListCalls,
	UpdateCalls, SoftDeleteCalls, RestoreCalls, SetStatusCalls int
}

func (m *mockRepo) Insert(ctx context.Context, in InsertParams) (*Client, error) {
	m.InsertCalls++
	if m.InsertHook != nil {
		return m.InsertHook(ctx, in)
	}
	return &Client{
		ID:             uuid.New(),
		Name:           in.Name,
		Slug:           in.Slug,
		TaxID:          in.TaxID,
		ContactEmail:   in.ContactEmail,
		ContactPhone:   in.ContactPhone,
		Address:        in.Address,
		Status:         in.Status,
	}, nil
}

func (m *mockRepo) GetByID(ctx context.Context, orgID, id uuid.UUID) (*Client, error) {
	m.GetByIDCalls++
	if m.GetByIDHook != nil {
		return m.GetByIDHook(ctx, orgID, id)
	}
	return nil, ErrClientNotFound
}

func (m *mockRepo) GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*Client, error) {
	m.GetBySlugCalls++
	if m.GetBySlugHook != nil {
		return m.GetBySlugHook(ctx, orgID, slug)
	}
	return nil, ErrClientNotFound
}

func (m *mockRepo) List(ctx context.Context, orgID uuid.UUID, f ListFilter) ([]*Client, int64, error) {
	m.ListCalls++
	if m.ListHook != nil {
		return m.ListHook(ctx, orgID, f)
	}
	return nil, 0, nil
}

func (m *mockRepo) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateParams) (*Client, error) {
	m.UpdateCalls++
	if m.UpdateHook != nil {
		return m.UpdateHook(ctx, orgID, id, in)
	}
	return &Client{ID: id, Name: in.Name, Status: in.Status}, nil
}

func (m *mockRepo) SoftDelete(ctx context.Context, orgID, id uuid.UUID) error {
	m.SoftDeleteCalls++
	if m.SoftDeleteHook != nil {
		return m.SoftDeleteHook(ctx, orgID, id)
	}
	return nil
}

func (m *mockRepo) Restore(ctx context.Context, orgID, id uuid.UUID) error {
	m.RestoreCalls++
	if m.RestoreHook != nil {
		return m.RestoreHook(ctx, orgID, id)
	}
	return nil
}

func (m *mockRepo) SetStatus(ctx context.Context, orgID, id uuid.UUID, status string) (*Client, error) {
	m.SetStatusCalls++
	if m.SetStatusHook != nil {
		return m.SetStatusHook(ctx, orgID, id, status)
	}
	return &Client{ID: id, Status: status}, nil
}
