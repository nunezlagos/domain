package ticket

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

type mockRepo struct {
	tickets   map[uuid.UUID]*Ticket
	history   map[uuid.UUID][]*StatusChange
	comments  map[uuid.UUID][]*Comment
	keySeq    int
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		tickets:  map[uuid.UUID]*Ticket{},
		history:  map[uuid.UUID][]*StatusChange{},
		comments: map[uuid.UUID][]*Comment{},
	}
}

func (m *mockRepo) Insert(_ context.Context, in CreateInput) (*Ticket, error) {
	m.keySeq++
	id := uuid.New()
	t := &Ticket{
		ID: id, OrganizationID: in.OrganizationID, ProjectID: in.ProjectID,
		Key: "TEST-" + itoaT(m.keySeq), Number: m.keySeq,
		Title: in.Title, DescriptionMD: in.DescriptionMD,
		IssueType: in.IssueType, Status: "backlog", Priority: in.Priority,
		ReporterID: in.ReporterID, Labels: in.Labels,
	}
	m.tickets[id] = t
	m.history[id] = []*StatusChange{{ID: uuid.New(), TicketID: id, ToStatus: "backlog", ChangedBy: in.ReporterID, Note: "created"}}
	return t, nil
}
func (m *mockRepo) Get(_ context.Context, _, id uuid.UUID) (*Ticket, error) {
	t, ok := m.tickets[id]
	if !ok {
		return nil, ErrNotFound
	}
	return t, nil
}
func (m *mockRepo) GetByKey(_ context.Context, _, _ uuid.UUID, _ string) (*Ticket, error) {
	return nil, ErrNotFound
}
func (m *mockRepo) List(_ context.Context, _ uuid.UUID, _ ListFilter) ([]*Ticket, int64, error) {
	return nil, 0, nil
}
func (m *mockRepo) Update(_ context.Context, _, id uuid.UUID, in UpdateInput) (*Ticket, error) {
	t, ok := m.tickets[id]
	if !ok {
		return nil, ErrNotFound
	}
	if in.Title != nil {
		t.Title = *in.Title
	}
	if in.Priority != nil {
		t.Priority = *in.Priority
	}
	return t, nil
}
func (m *mockRepo) ChangeStatus(_ context.Context, _, id uuid.UUID, toStatus string, changedBy uuid.UUID, note string) (*Ticket, error) {
	t, ok := m.tickets[id]
	if !ok {
		return nil, ErrNotFound
	}
	from := t.Status
	t.Status = toStatus
	m.history[id] = append(m.history[id], &StatusChange{
		TicketID: id, FromStatus: from, ToStatus: toStatus, ChangedBy: changedBy, Note: note,
	})
	return t, nil
}
func (m *mockRepo) SoftDelete(_ context.Context, _, _ uuid.UUID) error { return nil }
func (m *mockRepo) AddComment(_ context.Context, ticketID, authorID uuid.UUID, body string) (*Comment, error) {
	c := &Comment{ID: uuid.New(), TicketID: ticketID, AuthorID: authorID, BodyMD: body}
	m.comments[ticketID] = append(m.comments[ticketID], c)
	return c, nil
}
func (m *mockRepo) ListComments(_ context.Context, ticketID uuid.UUID) ([]*Comment, error) {
	return m.comments[ticketID], nil
}
func (m *mockRepo) StatusHistory(_ context.Context, ticketID uuid.UUID) ([]*StatusChange, error) {
	return m.history[ticketID], nil
}
func (m *mockRepo) LinkExternal(_ context.Context, _, id uuid.UUID, link ExternalLink) (*Ticket, error) {
	t, ok := m.tickets[id]
	if !ok {
		return nil, ErrNotFound
	}
	t.ExternalProvider = link.Provider
	t.ExternalID = link.ID
	t.ExternalURL = link.URL
	return t, nil
}
func (m *mockRepo) LinkIssue(_ context.Context, _, ticketID uuid.UUID, issueID *uuid.UUID) (*Ticket, error) {
	t, ok := m.tickets[ticketID]
	if !ok {
		return nil, ErrNotFound
	}
	t.LinkedIssueID = issueID
	return t, nil
}
func (m *mockRepo) UnlinkExternal(_ context.Context, _, id uuid.UUID) error {
	t, ok := m.tickets[id]
	if !ok {
		return ErrNotFound
	}
	t.ExternalProvider = ""
	t.ExternalID = ""
	t.ExternalURL = ""
	return nil
}

func itoaT(n int) string {
	if n == 0 {
		return "0"
	}
	out := ""
	for n > 0 {
		out = string(rune('0'+n%10)) + out
		n /= 10
	}
	return out
}

// ---------- tests ----------

func TestCreate_EmptyTitleErr(t *testing.T) {
	svc := NewService(newMockRepo())
	_, err := svc.Create(context.Background(), CreateInput{
		ProjectID: uuid.New(), Title: "  ",
	})
	if !errors.Is(err, ErrTitleRequired) {
		t.Errorf("err = %v, want ErrTitleRequired", err)
	}
}

func TestCreate_DefaultsTypeAndPriority(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)
	tk, err := svc.Create(context.Background(), CreateInput{
		OrganizationID: uuid.New(),
		ProjectID:      uuid.New(),
		Title:          "Fix login",
		ReporterID:     uuid.New(),
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if tk.IssueType != "task" {
		t.Errorf("IssueType = %q, want 'task' (default)", tk.IssueType)
	}
	if tk.Priority != "medium" {
		t.Errorf("Priority = %q, want 'medium' (default)", tk.Priority)
	}
	if tk.Status != "backlog" {
		t.Errorf("Status = %q, want 'backlog' (default)", tk.Status)
	}
}

func TestCreate_InvalidTypeErr(t *testing.T) {
	svc := NewService(newMockRepo())
	_, err := svc.Create(context.Background(), CreateInput{
		OrganizationID: uuid.New(), ProjectID: uuid.New(),
		Title:     "X",
		IssueType: "blooper",
		ReporterID: uuid.New(),
	})
	if !errors.Is(err, ErrInvalidType) {
		t.Errorf("err = %v, want ErrInvalidType", err)
	}
}

func TestCreate_InvalidPriorityErr(t *testing.T) {
	svc := NewService(newMockRepo())
	_, err := svc.Create(context.Background(), CreateInput{
		OrganizationID: uuid.New(), ProjectID: uuid.New(),
		Title: "X", Priority: "URGENT-NOW!",
		ReporterID: uuid.New(),
	})
	if !errors.Is(err, ErrInvalidPriority) {
		t.Errorf("err = %v, want ErrInvalidPriority", err)
	}
}

func TestChangeStatus_RecordsHistory(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)
	tk, _ := svc.Create(context.Background(), CreateInput{
		OrganizationID: uuid.New(), ProjectID: uuid.New(),
		Title: "X", ReporterID: uuid.New(),
	})
	_, err := svc.ChangeStatus(context.Background(), tk.OrganizationID, tk.ID, "in_progress", tk.ReporterID, "starting")
	if err != nil {
		t.Fatalf("ChangeStatus: %v", err)
	}
	hist, _ := svc.StatusHistory(context.Background(), tk.ID)
	if len(hist) != 2 {
		t.Fatalf("history len = %d, want 2 (created + in_progress)", len(hist))
	}
	if hist[1].ToStatus != "in_progress" || hist[1].FromStatus != "backlog" {
		t.Errorf("transition mal registrada: %+v", hist[1])
	}
}

func TestChangeStatus_InvalidStatusErr(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)
	tk, _ := svc.Create(context.Background(), CreateInput{
		OrganizationID: uuid.New(), ProjectID: uuid.New(),
		Title: "X", ReporterID: uuid.New(),
	})
	_, err := svc.ChangeStatus(context.Background(), tk.OrganizationID, tk.ID, "yolo", tk.ReporterID, "")
	if !errors.Is(err, ErrInvalidStatus) {
		t.Errorf("err = %v, want ErrInvalidStatus", err)
	}
}

func TestLinkExternal_ValidatesProvider(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)
	tk, _ := svc.Create(context.Background(), CreateInput{
		OrganizationID: uuid.New(), ProjectID: uuid.New(),
		Title: "X", ReporterID: uuid.New(),
	})
	_, err := svc.LinkExternal(context.Background(), tk.OrganizationID, tk.ID, ExternalLink{
		Provider: "monday", ID: "MON-1",
	})
	if !errors.Is(err, ErrInvalidProvider) {
		t.Errorf("err = %v, want ErrInvalidProvider", err)
	}
	// provider válido
	out, err := svc.LinkExternal(context.Background(), tk.OrganizationID, tk.ID, ExternalLink{
		Provider: "jira", ID: "ACME-1", URL: "https://x/ACME-1",
	})
	if err != nil {
		t.Fatalf("Link: %v", err)
	}
	if out.ExternalProvider != "jira" || out.ExternalID != "ACME-1" {
		t.Errorf("link no aplicado: %+v", out)
	}
}

func TestAddComment_EmptyBodyErr(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo)
	_, err := svc.AddComment(context.Background(), uuid.New(), uuid.New(), "   ")
	if !errors.Is(err, ErrBodyRequired) {
		t.Errorf("err = %v, want ErrBodyRequired", err)
	}
}
