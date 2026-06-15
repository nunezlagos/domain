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

// EventSink REQ-69: hook opcional para emitir eventos de cambios de
// ticket. El service no conoce de Bus/SSE — recibe solo este callback.
// Si nil, los hooks son no-op.
type EventSink func(topic string, t *Ticket, actor uuid.UUID, payload map[string]any)

type Service struct {
	repo   Repository
	events EventSink
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// SetEventSink inyecta el hook de eventos. Llamar en wireup tras crear
// el Service. Hacerlo idempotente (sobrescribe).
func (s *Service) SetEventSink(fn EventSink) {
	s.events = fn
}

func (s *Service) emit(topic string, t *Ticket, actor uuid.UUID, payload map[string]any) {
	if s.events == nil || t == nil {
		return
	}
	s.events(topic, t, actor, payload)
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

// checkLock REQ-63: si el ticket está lockeado por OTRO usuario y el
// lock no expiró, rechaza la operación. Reasignación pura (solo cambiar
// AssigneeID) sigue permitida — el dashboard lo necesita para "robar"
// un ticket lockeado vía /reassign.
func (s *Service) checkLock(ctx context.Context, orgID, id, actor uuid.UUID) error {
	t, err := s.repo.Get(ctx, orgID, id)
	if err != nil {
		return err
	}
	if t.LockedBy == nil || *t.LockedBy == actor {
		return nil
	}
	if t.LockedUntil == nil || !t.LockedUntil.After(timeNow()) {
		return nil // lock expirado, libre
	}
	return ErrLockedByOther
}

// Update sin actor: bypassa el lock check. Lo usan paths legacy (REST)
// donde aún no hay propagación del principal. Para enforcement del lock
// REQ-63, llamar UpdateAs.
func (s *Service) Update(ctx context.Context, orgID, id uuid.UUID, in UpdateInput) (*Ticket, error) {
	return s.UpdateAs(ctx, orgID, id, uuid.Nil, in)
}

func (s *Service) UpdateAs(ctx context.Context, orgID, id, actor uuid.UUID, in UpdateInput) (*Ticket, error) {
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
	// Si el caller solo está reasignando, no exigir holder del lock.
	// El dashboard reasigna para "quitar" tickets a quien los retiene.
	pureReassign := in.AssigneeID != nil &&
		in.Title == nil && in.DescriptionMD == nil && in.IssueType == nil &&
		in.Priority == nil && in.Labels == nil && in.ParentID == nil &&
		in.EstimatedHours == nil && in.ActualHours == nil && in.DueDate == nil
	if !pureReassign && actor != uuid.Nil {
		if err := s.checkLock(ctx, orgID, id, actor); err != nil {
			return nil, err
		}
	}
	t, err := s.repo.Update(ctx, orgID, id, in)
	if err == nil {
		topic := "ticket.update"
		if pureReassign {
			topic = "ticket.reassign"
		}
		s.emit(topic, t, actor, nil)
	}
	return t, err
}

func (s *Service) ChangeStatus(ctx context.Context, orgID, id uuid.UUID, toStatus string, changedBy uuid.UUID, note string) (*Ticket, error) {
	toStatus = strings.ToLower(strings.TrimSpace(toStatus))
	if _, ok := validStatuses[toStatus]; !ok {
		return nil, ErrInvalidStatus
	}
	if changedBy != uuid.Nil {
		if err := s.checkLock(ctx, orgID, id, changedBy); err != nil {
			return nil, err
		}
	}
	t, err := s.repo.ChangeStatus(ctx, orgID, id, toStatus, changedBy, note)
	if err == nil {
		s.emit("ticket.status", t, changedBy, map[string]any{
			"to": toStatus, "note": note,
		})
	}
	return t, err
}

// Claim adquiere el soft lock. REQ-63.
func (s *Service) Claim(ctx context.Context, orgID, ticketID, userID uuid.UUID, ttlMinutes int) (*Ticket, error) {
	if userID == uuid.Nil {
		return nil, ErrLockedByOther
	}
	t, err := s.repo.Claim(ctx, orgID, ticketID, userID, ttlMinutes)
	if err == nil {
		s.emit("ticket.claim", t, userID, map[string]any{
			"locked_until": t.LockedUntil,
		})
	}
	return t, err
}

// Release libera el lock. REQ-63.
func (s *Service) Release(ctx context.Context, orgID, ticketID, userID uuid.UUID) (*Ticket, error) {
	t, err := s.repo.Release(ctx, orgID, ticketID, userID)
	if err == nil {
		s.emit("ticket.release", t, userID, nil)
	}
	return t, err
}

// Reassign cambia el assignee bypaseando el lock check — equivalente a
// Update con solo AssigneeID pero más explícito. Pensado para el endpoint
// del dashboard "tomar este ticket" o "asignarle a Juan". REQ-63.
func (s *Service) Reassign(ctx context.Context, orgID, ticketID uuid.UUID, newAssignee *uuid.UUID) (*Ticket, error) {
	t, err := s.repo.Update(ctx, orgID, ticketID, UpdateInput{AssigneeID: newAssignee})
	if err == nil {
		s.emit("ticket.reassign", t, uuid.Nil, map[string]any{
			"new_assignee": newAssignee,
		})
	}
	return t, err
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

// LinkIssue vincula (o desvincula con issueID=nil) el ticket a una HU/issue
// del workflow SDD. REQ-56 Opción A — puente sin migrar datos.
func (s *Service) LinkIssue(ctx context.Context, orgID, ticketID uuid.UUID, issueID *uuid.UUID) (*Ticket, error) {
	return s.repo.LinkIssue(ctx, orgID, ticketID, issueID)
}

// BulkLinkExternal vincula N tickets a sus externals en una sola operación.
// Ideal para sync inicial cuando se conecta un proveedor externo (Jira)
// y hay que linkear 100+ tickets de una. REQ-58.
func (s *Service) BulkLinkExternal(ctx context.Context, orgID, projectID uuid.UUID, provider string, mappings []BulkLinkMapping) (*BulkLinkResult, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if _, ok := validProviders[provider]; !ok {
		return nil, ErrInvalidProvider
	}
	return s.repo.BulkLinkExternal(ctx, orgID, projectID, provider, mappings)
}

// FindByExternal: lookup por (provider, external_id). Útil para el
// webhook receiver de Jira/etc — encuentra el ticket local a actualizar.
// REQ-58.
func (s *Service) FindByExternal(ctx context.Context, orgID uuid.UUID, provider, externalID string) (*Ticket, error) {
	return s.repo.FindByExternal(ctx, orgID, provider, externalID)
}
