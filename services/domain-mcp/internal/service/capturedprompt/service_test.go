package capturedprompt

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// mockRepo implementa Repository en memoria para tests del Service.
type mockRepo struct {
	inserted  []InsertParams
	completed []CompleteTurnInput
	byID      map[uuid.UUID]*Prompt
}

func newMockRepo() *mockRepo { return &mockRepo{byID: map[uuid.UUID]*Prompt{}} }

func (m *mockRepo) Insert(_ context.Context, in InsertParams) (*Prompt, error) {
	m.inserted = append(m.inserted, in)
	id := uuid.New()
	p := &Prompt{
		ID: id, OrganizationID: in.OrganizationID, UserID: in.UserID,
		Content: in.Content, CharCount: in.CharCount,
		EstimatedTokensIn: (in.CharCount + 3) / 4, // mismo cálculo que el repo real
	}
	m.byID[id] = p
	return p, nil
}
func (m *mockRepo) Get(_ context.Context, _ uuid.UUID, id uuid.UUID) (*Prompt, error) {
	p, ok := m.byID[id]
	if !ok {
		return nil, ErrNotFound
	}
	return p, nil
}
func (m *mockRepo) List(_ context.Context, _ uuid.UUID, _ ListFilter) ([]*Prompt, int64, error) {
	return nil, 0, nil
}
func (m *mockRepo) CompleteTurn(_ context.Context, in CompleteTurnInput) (*Prompt, error) {
	m.completed = append(m.completed, in)
	p, ok := m.byID[in.PromptID]
	if !ok {
		return nil, ErrNotFound
	}
	p.ResponseChars = in.ResponseChars
	p.EstimatedTokensOut = (in.ResponseChars + 3) / 4
	if in.Model != "" {
		p.Model = in.Model
	}
	return p, nil
}
func (m *mockRepo) SummarizeByProject(_ context.Context, _, pid uuid.UUID) (*SessionUsage, error) {
	return &SessionUsage{ProjectID: &pid, Turns: 5, EstimatedTokensIn: 120, EstimatedTokensOut: 500}, nil
}

func TestCapture_EstimatedTokensIn_RatioFourToOne(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)
	in := CaptureInput{
		OrganizationID: uuid.New(),
		UserID:         uuid.New(),
		Content:        "abcd efgh ijkl", // 14 chars
	}
	p, err := svc.Capture(context.Background(), in)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if p.CharCount != 14 {
		t.Errorf("CharCount = %d, want 14", p.CharCount)
	}
	// ceil(14/4) = 4
	if p.EstimatedTokensIn != 4 {
		t.Errorf("EstimatedTokensIn = %d, want 4 (ceil(14/4))", p.EstimatedTokensIn)
	}
}

func TestCapture_EmptyContent_ReturnsErr(t *testing.T) {
	svc := NewService(newMockRepo())
	_, err := svc.Capture(context.Background(), CaptureInput{Content: "   \n  "})
	if !errors.Is(err, ErrEmptyContent) {
		t.Errorf("err = %v, want ErrEmptyContent", err)
	}
}

func TestCompleteTurn_EstimatesTokensOut(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)
	p, _ := svc.Capture(context.Background(), CaptureInput{
		OrganizationID: uuid.New(),
		UserID:         uuid.New(),
		Content:        "x", // 1 char
	})
	out, err := svc.CompleteTurn(context.Background(), CompleteTurnInput{
		OrganizationID: p.OrganizationID,
		PromptID:       p.ID,
		ResponseChars:  100,
		Model:          "claude-opus-4-7",
	})
	if err != nil {
		t.Fatalf("CompleteTurn: %v", err)
	}
	// ceil(100/4) = 25
	if out.EstimatedTokensOut != 25 {
		t.Errorf("EstimatedTokensOut = %d, want 25", out.EstimatedTokensOut)
	}
	if out.Model != "claude-opus-4-7" {
		t.Errorf("Model = %q, want claude-opus-4-7", out.Model)
	}
}

func TestCompleteTurn_NegativeResponseClampedToZero(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)
	p, _ := svc.Capture(context.Background(), CaptureInput{
		OrganizationID: uuid.New(),
		UserID:         uuid.New(),
		Content:        "a",
	})
	out, err := svc.CompleteTurn(context.Background(), CompleteTurnInput{
		OrganizationID: p.OrganizationID,
		PromptID:       p.ID,
		ResponseChars:  -50,
	})
	if err != nil {
		t.Fatalf("CompleteTurn: %v", err)
	}
	if out.ResponseChars != 0 {
		t.Errorf("ResponseChars = %d, want 0 (clamped)", out.ResponseChars)
	}
}

// REQ-42.3: SummarizeBySession removido (columna session_id dropeada).
func TestSummarizeByProject_DelegatesToRepo(t *testing.T) {
	svc := NewService(newMockRepo())
	u, err := svc.SummarizeByProject(context.Background(), uuid.New(), uuid.New())
	if err != nil {
		t.Fatalf("SummarizeByProject: %v", err)
	}
	if u.Turns != 5 || u.EstimatedTokensIn != 120 {
		t.Errorf("unexpected summary: %+v", u)
	}
}
