// Package ticket — REQ-51: sistema de tickets internos por proyecto.
// BD = source of truth. Sync opcional con Jira/GitHub/GitLab vía
// external_provider + external_id + external_url.
//
// NO confundir con package "issue" (que es para HUs / user stories del
// workflow SDD del orchestrator). Tickets aquí son tareas operativas
// del proyecto (bugs, features, requirements, mejoras).
package ticket

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound        = errors.New("ticket not found")
	ErrInvalidStatus   = errors.New("ticket: status inválido")
	ErrInvalidType     = errors.New("ticket: type inválido")
	ErrInvalidPriority = errors.New("ticket: priority inválida")
	ErrInvalidProvider = errors.New("ticket: external provider inválido")
	ErrTitleRequired   = errors.New("ticket: title requerido")
	ErrBodyRequired    = errors.New("ticket: body requerido")
	ErrProjectRequired = errors.New("ticket: project_id requerido")
	ErrSelfParent      = errors.New("ticket: parent_id no puede ser el propio ticket")
)

type Ticket struct {
	ID               uuid.UUID  `json:"id"`
	OrganizationID   uuid.UUID  `json:"organization_id"`
	ProjectID        uuid.UUID  `json:"project_id"`
	ClientID         *uuid.UUID `json:"client_id,omitempty"`
	Key              string     `json:"key"`
	Number           int        `json:"number"`
	Title            string     `json:"title"`
	DescriptionMD    string     `json:"description_md"`
	IssueType        string     `json:"issue_type"`
	Status           string     `json:"status"`
	Priority         string     `json:"priority"`
	AssigneeID       *uuid.UUID `json:"assignee_id,omitempty"`
	ReporterID       uuid.UUID  `json:"reporter_id"`
	Labels           []string   `json:"labels"`
	ExternalProvider string     `json:"external_provider,omitempty"`
	ExternalID       string     `json:"external_id,omitempty"`
	ExternalURL      string     `json:"external_url,omitempty"`
	ExternalSyncedAt *time.Time `json:"external_synced_at,omitempty"`
	ParentID         *uuid.UUID `json:"parent_id,omitempty"`
	LinkedIssueID    *uuid.UUID `json:"linked_issue_id,omitempty"`
	EstimatedHours   *float64   `json:"estimated_hours,omitempty"`
	ActualHours      *float64   `json:"actual_hours,omitempty"`
	DueDate          *time.Time `json:"due_date,omitempty"`
	StartedAt        *time.Time `json:"started_at,omitempty"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
}

type Comment struct {
	ID         uuid.UUID  `json:"id"`
	TicketID   uuid.UUID  `json:"ticket_id"`
	AuthorID   uuid.UUID  `json:"author_id"`
	BodyMD     string     `json:"body_md"`
	ExternalID string     `json:"external_id,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
}

type StatusChange struct {
	ID         uuid.UUID `json:"id"`
	TicketID   uuid.UUID `json:"ticket_id"`
	FromStatus string    `json:"from_status,omitempty"`
	ToStatus   string    `json:"to_status"`
	ChangedBy  uuid.UUID `json:"changed_by"`
	Note       string    `json:"note,omitempty"`
	ChangedAt  time.Time `json:"changed_at"`
}

type CreateInput struct {
	OrganizationID uuid.UUID
	ProjectID      uuid.UUID
	ClientID       *uuid.UUID
	ProjectSlug    string
	Title          string
	DescriptionMD  string
	IssueType      string
	Priority       string
	AssigneeID     *uuid.UUID
	ReporterID     uuid.UUID
	Labels         []string
	ParentID       *uuid.UUID
	EstimatedHours *float64
	DueDate        *time.Time
}

type UpdateInput struct {
	Title          *string
	DescriptionMD  *string
	IssueType      *string
	Priority       *string
	AssigneeID     *uuid.UUID
	Labels         *[]string
	ParentID       *uuid.UUID
	EstimatedHours *float64
	ActualHours    *float64
	DueDate        *time.Time
}

type ListFilter struct {
	ProjectID  *uuid.UUID
	Status     string
	IssueType  string
	Priority   string
	AssigneeID *uuid.UUID
	ReporterID *uuid.UUID
	ParentID   *uuid.UUID
	Label      string
	Query      string
	Limit      int
	Offset     int
}

type ExternalLink struct {
	Provider string
	ID       string
	URL      string
}

type Repository interface {
	LinkIssue(ctx context.Context, orgID, ticketID uuid.UUID, issueID *uuid.UUID) (*Ticket, error)
	Insert(ctx context.Context, in CreateInput) (*Ticket, error)
	Get(ctx context.Context, orgID, id uuid.UUID) (*Ticket, error)
	GetByKey(ctx context.Context, orgID, projectID uuid.UUID, key string) (*Ticket, error)
	List(ctx context.Context, orgID uuid.UUID, filter ListFilter) ([]*Ticket, int64, error)
	Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput) (*Ticket, error)
	ChangeStatus(ctx context.Context, orgID, id uuid.UUID, toStatus string, changedBy uuid.UUID, note string) (*Ticket, error)
	SoftDelete(ctx context.Context, orgID, id uuid.UUID) error
	AddComment(ctx context.Context, ticketID, authorID uuid.UUID, body string) (*Comment, error)
	ListComments(ctx context.Context, ticketID uuid.UUID) ([]*Comment, error)
	StatusHistory(ctx context.Context, ticketID uuid.UUID) ([]*StatusChange, error)
	LinkExternal(ctx context.Context, orgID, id uuid.UUID, link ExternalLink) (*Ticket, error)
	UnlinkExternal(ctx context.Context, orgID, id uuid.UUID) error
}
