package projectrepo

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type mockRepo struct {
	repos     map[uuid.UUID]*Repo
	byProject map[uuid.UUID][]*Repo
}

func newMockRepo() *mockRepo {
	return &mockRepo{repos: map[uuid.UUID]*Repo{}, byProject: map[uuid.UUID][]*Repo{}}
}

func (m *mockRepo) Insert(_ context.Context, in InsertParams) (*Repo, error) {
	id := uuid.New()
	r := &Repo{
		ID: id, ProjectID: in.ProjectID,
		Name: in.Name, URL: in.URL, BranchDefault: in.BranchDefault,
		Kind: in.Kind, IsDefault: in.IsDefault, Workflow: in.Workflow, Notes: in.Notes,
	}
	m.repos[id] = r
	m.byProject[in.ProjectID] = append(m.byProject[in.ProjectID], r)
	return r, nil
}
func (m *mockRepo) List(_ context.Context, _, projectID uuid.UUID) ([]*Repo, error) {
	return m.byProject[projectID], nil
}
func (m *mockRepo) Get(_ context.Context, _, id uuid.UUID) (*Repo, error) {
	r, ok := m.repos[id]
	if !ok {
		return nil, ErrNotFound
	}
	return r, nil
}
func (m *mockRepo) GetByName(_ context.Context, _, _ uuid.UUID, _ string) (*Repo, error) {
	return nil, ErrNotFound
}
func (m *mockRepo) Update(_ context.Context, _, id uuid.UUID, in UpdateParams) (*Repo, error) {
	r, ok := m.repos[id]
	if !ok {
		return nil, ErrNotFound
	}
	if in.URL != nil {
		r.URL = *in.URL
	}
	if in.Workflow != nil {
		r.Workflow = *in.Workflow
	}
	return r, nil
}
func (m *mockRepo) SetDefault(_ context.Context, _, id uuid.UUID) (*Repo, error) {
	target, ok := m.repos[id]
	if !ok {
		return nil, ErrNotFound
	}

	for _, r := range m.byProject[target.ProjectID] {
		if r.ID != id {
			r.IsDefault = false
		}
	}
	target.IsDefault = true
	return target, nil
}
func (m *mockRepo) SoftDelete(_ context.Context, _, id uuid.UUID) error {
	if _, ok := m.repos[id]; !ok {
		return ErrNotFound
	}
	delete(m.repos, id)
	return nil
}



func TestAdd_FirstRepoBecomesDefault(t *testing.T) {
	svc := NewService(newMockRepo())
	r, err := svc.Add(context.Background(), AddInput{
		ProjectID: uuid.New(),
		Name:      "origin", URL: "https://github.com/x.git",
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if !r.IsDefault {
		t.Error("primer repo debería quedar IsDefault=true automáticamente")
	}
}

func TestAdd_RequiresNameAndURL(t *testing.T) {
	svc := NewService(newMockRepo())
	_, err := svc.Add(context.Background(), AddInput{Name: "", URL: "x"})
	if !errors.Is(err, ErrNameRequired) {
		t.Errorf("err = %v, want ErrNameRequired", err)
	}
	_, err = svc.Add(context.Background(), AddInput{Name: "origin", URL: ""})
	if !errors.Is(err, ErrURLRequired) {
		t.Errorf("err = %v, want ErrURLRequired", err)
	}
}

func TestAdd_RejectsInvalidWorkflow(t *testing.T) {
	svc := NewService(newMockRepo())
	_, err := svc.Add(context.Background(), AddInput{
		ProjectID: uuid.New(),
		Name:      "origin", URL: "x", Workflow: "wild-west",
	})
	if !errors.Is(err, ErrInvalidWorkflow) {
		t.Errorf("err = %v, want ErrInvalidWorkflow", err)
	}
}

func TestAdd_SecondRepoNotDefaultByDefault(t *testing.T) {
	svc := NewService(newMockRepo())
	projID := uuid.New()
	_, _ = svc.Add(context.Background(), AddInput{
		ProjectID: projID,
		Name:      "origin", URL: "https://github.com/x.git",
	})
	r2, err := svc.Add(context.Background(), AddInput{
		ProjectID: projID,
		Name:      "mirror", URL: "git@gitlab:x.git",
	})
	if err != nil {
		t.Fatalf("Add mirror: %v", err)
	}
	if r2.IsDefault {
		t.Error("segundo repo NO debería ser default a menos que IsDefault=true explícito")
	}
}

func TestSetDefault_SwitchesAtomically(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)
	orgID, projID := uuid.New(), uuid.New()
	r1, _ := svc.Add(context.Background(), AddInput{
		ProjectID: projID,
		Name:      "origin", URL: "https://github.com/x.git",
	})
	r2, _ := svc.Add(context.Background(), AddInput{
		ProjectID: projID,
		Name:      "mirror", URL: "git@gitlab:x.git",
	})
	if !r1.IsDefault || r2.IsDefault {
		t.Fatalf("estado inicial mal: r1.default=%v r2.default=%v", r1.IsDefault, r2.IsDefault)
	}

	_, err := svc.SetDefault(context.Background(), orgID, r2.ID)
	if err != nil {
		t.Fatalf("SetDefault: %v", err)
	}
	if r1.IsDefault {
		t.Error("r1 debería haber dejado de ser default")
	}
	if !r2.IsDefault {
		t.Error("r2 debería ser default ahora")
	}
}
