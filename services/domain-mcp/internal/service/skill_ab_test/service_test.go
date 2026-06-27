package skill_ab_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

// fakeRepo implementa Repository en memoria para testear el Service sin DB.
type fakeRepo struct {
	running   []*ABTest
	results   map[uuid.UUID][]VariantResult
	declared  map[uuid.UUID]string
	confs     map[uuid.UUID]*float64
	pinned    map[uuid.UUID]int // skillID -> version
	slugToID  map[string]uuid.UUID
	createErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		results:  map[uuid.UUID][]VariantResult{},
		declared: map[uuid.UUID]string{},
		confs:    map[uuid.UUID]*float64{},
		pinned:   map[uuid.UUID]int{},
		slugToID: map[string]uuid.UUID{},
	}
}

func (f *fakeRepo) Create(_ context.Context, p CreateParams) (*ABTest, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	t := &ABTest{ID: uuid.New(), SkillSlug: p.SkillSlug, VersionA: p.VersionA,
		VersionB: p.VersionB, TrafficSplitA: p.TrafficSplitA, MinInvocations: p.MinInvocations,
		AutoApplyWinner: p.AutoApplyWinner, Status: StatusRunning}
	f.running = append(f.running, t)
	return t, nil
}
func (f *fakeRepo) Get(_ context.Context, id uuid.UUID) (*ABTest, error) {
	for _, t := range f.running {
		if t.ID == id {
			return t, nil
		}
	}
	return nil, nil
}
func (f *fakeRepo) GetRunningBySlug(_ context.Context, slug string) (*ABTest, error) {
	for _, t := range f.running {
		if t.SkillSlug == slug && t.Status == StatusRunning {
			return t, nil
		}
	}
	return nil, nil
}
func (f *fakeRepo) ListRunning(_ context.Context) ([]*ABTest, error) {
	out := []*ABTest{}
	for _, t := range f.running {
		if t.Status == StatusRunning {
			out = append(out, t)
		}
	}
	return out, nil
}
func (f *fakeRepo) Start(_ context.Context, _ uuid.UUID) error { return nil }
func (f *fakeRepo) DeclareWinner(_ context.Context, id uuid.UUID, w string, c *float64) error {
	f.declared[id] = w
	f.confs[id] = c
	for _, t := range f.running {
		if t.ID == id {
			t.Status = StatusCompleted
		}
	}
	return nil
}
func (f *fakeRepo) Cancel(_ context.Context, id uuid.UUID) error {
	for _, t := range f.running {
		if t.ID == id {
			t.Status = StatusCancelled
		}
	}
	return nil
}
func (f *fakeRepo) GetResults(_ context.Context, id uuid.UUID) ([]VariantResult, error) {
	return f.results[id], nil
}
func (f *fakeRepo) IncrementResult(_ context.Context, _ uuid.UUID, _ Variant, _ bool) error {
	return nil
}
func (f *fakeRepo) UpsertResult(_ context.Context, _ uuid.UUID, _ VariantResult) error { return nil }
func (f *fakeRepo) SkillIDBySlug(_ context.Context, slug string) (uuid.UUID, error) {
	return f.slugToID[slug], nil
}
func (f *fakeRepo) PinSkillVersion(_ context.Context, skillID uuid.UUID, version int) error {
	f.pinned[skillID] = version
	return nil
}

// TestCreateRejectsSameVersions valida la regla version_a != version_b.
func TestCreateRejectsSameVersions(t *testing.T) {
	s := NewService(newFakeRepo())
	_, err := s.Create(bg(), CreateParams{SkillSlug: "x", VersionA: 2, VersionB: 2})
	if err != ErrInvalidVersions {
		t.Fatalf("esperaba ErrInvalidVersions, dio %v", err)
	}
}

// TestCreateRejectsDuplicateRunning valida el opt-in unico por slug.
func TestCreateRejectsDuplicateRunning(t *testing.T) {
	repo := newFakeRepo()
	s := NewService(repo)
	if _, err := s.Create(bg(), CreateParams{SkillSlug: "x", VersionA: 1, VersionB: 2}); err != nil {
		t.Fatalf("primer create fallo: %v", err)
	}
	_, err := s.Create(bg(), CreateParams{SkillSlug: "x", VersionA: 3, VersionB: 4})
	if err != ErrAlreadyRunning {
		t.Fatalf("esperaba ErrAlreadyRunning, dio %v", err)
	}
}

// TestAnalyzeRunningDeclaresWinnerNoAutoApply: por default NO pinea (solo declara).
func TestAnalyzeRunningDeclaresWinnerNoAutoApply(t *testing.T) {
	repo := newFakeRepo()
	test := &ABTest{ID: uuid.New(), SkillSlug: "x", VersionA: 5, VersionB: 9,
		MinInvocations: 100, AutoApplyWinner: false, Status: StatusRunning}
	repo.running = []*ABTest{test}
	repo.results[test.ID] = []VariantResult{
		{Version: "a", InvocationsCount: 200, SuccessCount: 180},
		{Version: "b", InvocationsCount: 200, SuccessCount: 120},
	}
	skillID := uuid.New()
	repo.slugToID["x"] = skillID

	s := NewService(repo)
	out, err := s.AnalyzeRunning(bg(), NewAnalyzer(0.05), false)
	if err != nil {
		t.Fatalf("AnalyzeRunning fallo: %v", err)
	}
	if len(out) != 1 || !out[0].Declared {
		t.Fatalf("esperaba 1 declarado, dio %+v", out)
	}
	if repo.declared[test.ID] != WinnerA {
		t.Fatalf("esperaba ganador A, dio %q", repo.declared[test.ID])
	}
	if _, pinned := repo.pinned[skillID]; pinned {
		t.Fatal("auto_apply=false NO deberia pinear ninguna version")
	}
	if out[0].PinApplied {
		t.Fatal("PinApplied deberia ser false sin auto_apply")
	}
}

// TestAnalyzeRunningAutoAppliesWinner: con auto_apply (por-test) pinea la version
// ganadora en el skill. Verifica que se usa skills.pinned_version (NUNCA org_id).
func TestAnalyzeRunningAutoAppliesWinner(t *testing.T) {
	repo := newFakeRepo()
	test := &ABTest{ID: uuid.New(), SkillSlug: "x", VersionA: 5, VersionB: 9,
		MinInvocations: 100, AutoApplyWinner: true, Status: StatusRunning}
	repo.running = []*ABTest{test}
	repo.results[test.ID] = []VariantResult{
		{Version: "a", InvocationsCount: 200, SuccessCount: 120},
		{Version: "b", InvocationsCount: 200, SuccessCount: 185},
	}
	skillID := uuid.New()
	repo.slugToID["x"] = skillID

	s := NewService(repo)
	out, err := s.AnalyzeRunning(bg(), NewAnalyzer(0.05), false)
	if err != nil {
		t.Fatalf("AnalyzeRunning fallo: %v", err)
	}
	if repo.declared[test.ID] != WinnerB {
		t.Fatalf("esperaba ganador B, dio %q", repo.declared[test.ID])
	}
	if got := repo.pinned[skillID]; got != 9 {
		t.Fatalf("esperaba pin a version 9 (B), dio %d", got)
	}
	if !out[0].PinApplied {
		t.Fatal("PinApplied deberia ser true con auto_apply")
	}
}

// TestAnalyzeRunningSmallSampleNotDeclared: muestra chica -> no se declara.
func TestAnalyzeRunningSmallSampleNotDeclared(t *testing.T) {
	repo := newFakeRepo()
	test := &ABTest{ID: uuid.New(), SkillSlug: "x", VersionA: 1, VersionB: 2,
		MinInvocations: 100, Status: StatusRunning}
	repo.running = []*ABTest{test}
	repo.results[test.ID] = []VariantResult{
		{Version: "a", InvocationsCount: 10, SuccessCount: 9},
		{Version: "b", InvocationsCount: 10, SuccessCount: 2},
	}
	s := NewService(repo)
	out, err := s.AnalyzeRunning(bg(), NewAnalyzer(0.05), false)
	if err != nil {
		t.Fatalf("AnalyzeRunning fallo: %v", err)
	}
	if out[0].Declared {
		t.Fatal("muestra chica NO deberia declararse")
	}
	if _, ok := repo.declared[test.ID]; ok {
		t.Fatal("no deberia haber declarado ganador")
	}
}

// TestAnalyzeRunningInconclusiveNoPin: empate -> declara inconclusive, no pinea
// aunque auto_apply este on.
func TestAnalyzeRunningInconclusiveNoPin(t *testing.T) {
	repo := newFakeRepo()
	test := &ABTest{ID: uuid.New(), SkillSlug: "x", VersionA: 1, VersionB: 2,
		MinInvocations: 100, AutoApplyWinner: true, Status: StatusRunning}
	repo.running = []*ABTest{test}
	repo.results[test.ID] = []VariantResult{
		{Version: "a", InvocationsCount: 1000, SuccessCount: 800},
		{Version: "b", InvocationsCount: 1000, SuccessCount: 800},
	}
	skillID := uuid.New()
	repo.slugToID["x"] = skillID
	s := NewService(repo)
	out, err := s.AnalyzeRunning(bg(), NewAnalyzer(0.05), true)
	if err != nil {
		t.Fatalf("AnalyzeRunning fallo: %v", err)
	}
	if repo.declared[test.ID] != WinnerInconclusive {
		t.Fatalf("esperaba inconclusive, dio %q", repo.declared[test.ID])
	}
	if _, pinned := repo.pinned[skillID]; pinned {
		t.Fatal("inconclusive NO deberia pinear ninguna version")
	}
	_ = out
}
