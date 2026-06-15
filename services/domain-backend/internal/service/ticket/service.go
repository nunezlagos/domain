package ticket

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

var validTypes = map[string]struct{}{
	"bug": {}, "feature": {}, "requirement": {}, "task": {},
	"epic": {}, "improvement": {}, "spike": {},
}
var validStatuses = map[string]struct{}{
	"backlog": {}, "todo": {}, "in_progress": {}, "in_review": {},
	"blocked": {}, "done": {}, "cancelled": {},
}
var validPriorities = map[string]struct{}{
	"trivial": {}, "low": {}, "medium": {}, "high": {}, "critical": {},
}
var validProviders = map[string]struct{}{
	"jira": {}, "github": {}, "gitlab": {}, "linear": {}, "azure_devops": {},
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*Ticket, error) {
	in.Title = strings.TrimSpace(in.Title)
	if in.Title == "" {
		return nil, ErrTitleRequired
	}
	if in.ProjectID == uuid.Nil {
		return nil, ErrProjectRequired
	}
	if in.IssueType == "" {
		in.IssueType = "task"
	}
	in.IssueType = strings.ToLower(strings.TrimSpace(in.IssueType))
	if _, ok := validTypes[in.IssueType]; !ok {
		return nil, ErrInvalidType
	}
	if in.Priority == "" {
		in.Priority = "medium"
	}
	in.Priority = strings.ToLower(strings.TrimSpace(in.Priority))
	if _, ok := validPriorities[in.Priority]; !ok {
		return nil, ErrInvalidPriority
	}
	if in.Labels == nil {
		in.Labels = []string{}
	}
	return s.repo.Insert(ctx, in)
}

func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (*Ticket, error) {
	return s.repo.Get(ctx, orgID, id)
}

func (s *Service) GetByKey(ctx context.Context, orgID, projectID uuid.UUID, key string) (*Ticket, error) {
	return s.repo.GetByKey(ctx, orgID, projectID, key)
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID, filter ListFilter) ([]*Ticket, int64, error) {
	return s.repo.List(ctx, orgID, filter)
}

func (s *Service) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput) (*Ticket, error) {
	if in.IssueType != nil {
		v := strings.ToLower(strings.TrimSpace(*in.IssueType))
		if _, ok := validTypes[v]; !ok {
			return nil, ErrInvalidType
		}
		in.IssueType = &v
	}
	if in.Priority != nil {
		v := strings.ToLower(strings.TrimSpace(*in.Priority))
		if _, ok := validPriorities[v]; !ok {
			return nil, ErrInvalidPriority
		}
		in.Priority = &v
	}
	return s.repo.Update(ctx, orgID, id, in)
}

func (s *Service) ChangeStatus(ctx context.Context, orgID, id uuid.UUID, toStatus string, changedBy uuid.UUID, note string) (*Ticket, error) {
	toStatus = strings.ToLower(strings.TrimSpace(toStatus))
	if _, ok := validStatuses[toStatus]; !ok {
		return nil, ErrInvalidStatus
	}
	return s.repo.ChangeStatus(ctx, orgID, id, toStatus, changedBy, note)
}

func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return s.repo.SoftDelete(ctx, orgID, id)
}

func (s *Service) AddComment(ctx context.Context, ticketID, authorID uuid.UUID, body string) (*Comment, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, ErrBodyRequired
	}
	return s.repo.AddComment(ctx, ticketID, authorID, body)
}

func (s *Service) ListComments(ctx context.Context, ticketID uuid.UUID) ([]*Comment, error) {
	return s.repo.ListComments(ctx, ticketID)
}

func (s *Service) StatusHistory(ctx context.Context, ticketID uuid.UUID) ([]*StatusChange, error) {
	return s.repo.StatusHistory(ctx, ticketID)
}

func (s *Service) LinkExternal(ctx context.Context, orgID, id uuid.UUID, link ExternalLink) (*Ticket, error) {
	link.Provider = strings.ToLower(strings.TrimSpace(link.Provider))
	if link.Provider == "" || link.ID == "" {
		return nil, ErrInvalidProvider
	}
	if _, ok := validProviders[link.Provider]; !ok {
		return nil, ErrInvalidProvider
	}
	return s.repo.LinkExternal(ctx, orgID, id, link)
}

func (s *Service) UnlinkExternal(ctx context.Context, orgID, id uuid.UUID) error {
	return s.repo.UnlinkExternal(ctx, orgID, id)
}
