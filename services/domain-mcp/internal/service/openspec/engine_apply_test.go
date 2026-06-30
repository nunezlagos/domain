package openspec

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	issuesvc "nunezlagos/domain/internal/service/issue"
	specsvc "nunezlagos/domain/internal/service/spec"
)

// fakeSpecWriter cuenta cuántas versiones se crearon para verificar que un md
// vacío NO genera versión (BUG 1).
type fakeSpecWriter struct {
	proposals int
	designs   int
}

func (f *fakeSpecWriter) CreateProposal(_ context.Context, _ uuid.UUID, _, _, _, _, _ string) (*specsvc.Proposal, error) {
	f.proposals++
	return &specsvc.Proposal{}, nil
}

func (f *fakeSpecWriter) CreateDesign(_ context.Context, _ uuid.UUID, _ *uuid.UUID, _, _, _, _, _ string) (*specsvc.Design, error) {
	f.designs++
	return &specsvc.Design{}, nil
}

// fakeIssueWriter registra remove/add para verificar atomicidad (BUG 2): si la
// validación falla, removed debe quedar en 0.
type fakeIssueWriter struct {
	removed   int
	added     int
	failAddOn int // si >0, AddScenario falla en la N-ésima llamada
}

func (f *fakeIssueWriter) Update(_ context.Context, _ string, _, _, _, _ *string) (*issuesvc.Issue, error) {
	return &issuesvc.Issue{}, nil
}

func (f *fakeIssueWriter) AddScenario(_ context.Context, _ string, _ issuesvc.Scenario) (*issuesvc.Scenario, error) {
	f.added++
	if f.failAddOn > 0 && f.added == f.failAddOn {
		return nil, errors.New("insert falló")
	}
	return &issuesvc.Scenario{}, nil
}

func (f *fakeIssueWriter) RemoveScenario(_ context.Context, _ uuid.UUID) error {
	f.removed++
	return nil
}

// --- BUG 1: md vacío/borrado no debe versionar ---

func TestApplyProposal_EmptyMdRejected(t *testing.T) {
	sw := &fakeSpecWriter{}
	e := &Engine{SpecW: sw}
	for _, md := range []string{"", "   ", "\n\t\n"} {
		err := e.applyProposal(context.Background(), uuid.New(), md)
		if !errors.Is(err, ErrEmptyDoc) {
			t.Fatalf("md=%q: esperaba ErrEmptyDoc, got %v", md, err)
		}
	}
	if sw.proposals != 0 {
		t.Fatalf("md vacío NO debe crear proposal, se crearon %d", sw.proposals)
	}
}

func TestApplyDesign_EmptyMdRejected(t *testing.T) {
	sw := &fakeSpecWriter{}
	e := &Engine{SpecW: sw}
	if err := e.applyDesign(context.Background(), uuid.New(), ""); !errors.Is(err, ErrEmptyDoc) {
		t.Fatalf("esperaba ErrEmptyDoc, got %v", err)
	}
	if sw.designs != 0 {
		t.Fatalf("md vacío NO debe crear design, se crearon %d", sw.designs)
	}
}

func TestApplyProposal_NonEmptyVersions(t *testing.T) {
	sw := &fakeSpecWriter{}
	e := &Engine{SpecW: sw}
	md := "# Título\n\n## Por qué\n\nrazón\n"
	if err := e.applyProposal(context.Background(), uuid.New(), md); err != nil {
		t.Fatalf("md no vacío: error inesperado %v", err)
	}
	if sw.proposals != 1 {
		t.Fatalf("md válido debe crear 1 proposal, got %d", sw.proposals)
	}
}

// --- BUG 2: applyScenarios atómico (no borra si la validación falla) ---

func issueWithScenarios(n int) *issuesvc.Issue {
	iss := &issuesvc.Issue{Slug: "hu-1"}
	for i := 0; i < n; i++ {
		iss.Scenarios = append(iss.Scenarios, issuesvc.Scenario{ID: uuid.New()})
	}
	return iss
}

func TestApplyScenarios_EmptyMdDoesNotDelete(t *testing.T) {
	iw := &fakeIssueWriter{}
	e := &Engine{IssuesW: iw}
	err := e.applyScenarios(context.Background(), issueWithScenarios(3), "")
	if !errors.Is(err, ErrEmptyDoc) {
		t.Fatalf("esperaba ErrEmptyDoc, got %v", err)
	}
	if iw.removed != 0 {
		t.Fatalf("md vacío NO debe borrar escenarios, se borraron %d", iw.removed)
	}
}

func TestApplyScenarios_InvalidNewDoesNotDelete(t *testing.T) {
	// spec.md con Feature y nombre pero sin Given/Then -> inválido.
	md := "# Mi Feature\n\n## Scenario: incompleto\n\n- When algo\n"
	iw := &fakeIssueWriter{}
	e := &Engine{IssuesW: iw}
	if err := e.applyScenarios(context.Background(), issueWithScenarios(2), md); err == nil {
		t.Fatal("esperaba error de validación, got nil")
	}
	if iw.removed != 0 {
		t.Fatalf("validación falló: NO debe borrar viejos, se borraron %d", iw.removed)
	}
	if iw.added != 0 {
		t.Fatalf("validación falló: NO debe insertar, se insertaron %d", iw.added)
	}
}

func TestApplyScenarios_NoScenariosDoesNotDelete(t *testing.T) {
	// md sin ningún escenario parseable (sólo texto) -> no reemplaza.
	md := "# Feature sin escenarios\n\nblah blah\n"
	iw := &fakeIssueWriter{}
	e := &Engine{IssuesW: iw}
	if err := e.applyScenarios(context.Background(), issueWithScenarios(2), md); err == nil {
		t.Fatal("esperaba error por 0 escenarios, got nil")
	}
	if iw.removed != 0 {
		t.Fatalf("0 escenarios válidos: NO debe borrar, se borraron %d", iw.removed)
	}
}

func TestApplyScenarios_ValidReplaces(t *testing.T) {
	md := "# Mi Feature\n\n## Scenario: ok\n\n- Given precondición\n- When acción\n- Then resultado\n"
	iw := &fakeIssueWriter{}
	e := &Engine{IssuesW: iw}
	if err := e.applyScenarios(context.Background(), issueWithScenarios(2), md); err != nil {
		t.Fatalf("escenario válido: error inesperado %v", err)
	}
	if iw.removed != 2 {
		t.Fatalf("debe borrar los 2 viejos, borró %d", iw.removed)
	}
	if iw.added != 1 {
		t.Fatalf("debe insertar 1 nuevo, insertó %d", iw.added)
	}
}
