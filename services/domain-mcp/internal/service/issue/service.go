// Package issue — user stories (HU) con escenarios Gherkin, scopeadas por proyecto.
//
//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
package issue

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/service/issue/issuedb"
)

const (
	StatusProposed    = "proposed"
	StatusActive      = "active"
	StatusImplemented = "implemented"
	StatusArchived    = "archived"

	PriorityLow      = "low"
	PriorityMedium   = "medium"
	PriorityHigh     = "high"
	PriorityCritical = "critical"
)

var (
	ErrNotFound        = errors.New("user story not found")
	ErrSlugTaken       = errors.New("user story slug already taken")
	ErrSlugInvalid     = errors.New("slug must match issue-NN.N-* pattern")
	ErrReqNotFound     = errors.New("requirement not found")
	ErrScenarioInvalid = errors.New("scenario validation failed")
	ErrInvalidStatus   = errors.New("invalid status")
	ErrInvalidPriority = errors.New("invalid priority")

	ErrProjectIDRequired = errors.New("project_id required")
)

var reIssueSlug = regexp.MustCompile(`^issue-\d+\.\d+(-[a-z0-9-]+)?$`)
var validStatuses = map[string]bool{StatusProposed: true, StatusActive: true, StatusImplemented: true, StatusArchived: true}
var validPriorities = map[string]bool{PriorityLow: true, PriorityMedium: true, PriorityHigh: true, PriorityCritical: true}

// Issue snapshot.
type Issue struct {
	ID          uuid.UUID  `json:"id"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	Description *string    `json:"description,omitempty"`
	Status      string     `json:"status"`
	Priority    string     `json:"priority"`
	ReqID       uuid.UUID  `json:"req_id"`
	ProjectID   *uuid.UUID `json:"project_id,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Scenarios   []Scenario `json:"scenarios,omitempty"`
}

// Scenario un escenario Gherkin estructurado.
type Scenario struct {
	ID        uuid.UUID `json:"id"`
	HuID      uuid.UUID `json:"issue_id"`
	Feature   string    `json:"feature"`
	Scenario  string    `json:"scenario"`
	Given     []string  `json:"given"`
	When      string    `json:"when"`
	Then      []string  `json:"then"`
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
}

// UserStoryFilter opcional para List.
type UserStoryFilter struct {
	Status   string
	Priority string
	ReqSlug  string
	Limit    int
	Offset   int
}

// Service CRUD para user stories.
type Service struct {
	Pool  *pgxpool.Pool
	Audit *audit.PGRecorder
}

func (s *Service) q() *issuedb.Queries { return issuedb.New(s.Pool) }

// Create inserta una HU con sus escenarios.
func (s *Service) Create(ctx context.Context, slug, title, description, status, priority, reqSlug string, scenarios []Scenario) (*Issue, error) {
	if !reIssueSlug.MatchString(slug) {
		return nil, ErrSlugInvalid
	}
	if title == "" {
		return nil, errors.New("title required")
	}
	if status == "" {
		status = StatusProposed
	}
	if !validStatuses[status] {
		return nil, ErrInvalidStatus
	}
	if priority == "" {
		priority = PriorityMedium
	}
	if !validPriorities[priority] {
		return nil, ErrInvalidPriority
	}
	if err := validateScenarios(scenarios); err != nil {
		return nil, err
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := issuedb.New(tx)

	// project_id heredado del requirement padre (scoping por proyecto).
	req, err := q.GetRequirementForIssue(ctx, reqSlug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReqNotFound
		}
		return nil, fmt.Errorf("find req: %w", err)
	}
	if req.ProjectID == uuid.Nil {
		return nil, ErrProjectIDRequired
	}

	var desc *string
	if description != "" {
		desc = &description
	}

	row, err := q.InsertIssue(ctx, issuedb.InsertIssueParams{
		Slug:        slug,
		Title:       title,
		Description: desc,
		Status:      status,
		Priority:    priority,
		ReqID:       req.ID,
		ProjectID:   req.ProjectID,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrSlugTaken
		}
		return nil, fmt.Errorf("insert user_story: %w", err)
	}

	hu := toIssue(row)
	for i, sc := range scenarios {
		scRow, err := q.InsertScenario(ctx, issuedb.InsertScenarioParams{
			IssueID:   hu.ID,
			ProjectID: req.ProjectID,
			Feature:   sc.Feature,
			Scenario:  sc.Scenario,
			Given:     sc.Given,
			WhenText:  sc.When,
			ThenRows:  sc.Then,
			Position:  int32(i),
		})
		if err != nil {
			return nil, fmt.Errorf("insert scenario %d: %w", i, err)
		}
		hu.Scenarios = append(hu.Scenarios, toScenario(scRow))
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorType:  audit.ActorSystem,
			Action:     "user_story.created",
			EntityType: "user_story",
			EntityID:   &hu.ID,
			NewValues:  map[string]any{"slug": slug, "title": title, "req_id": req.ID.String()},
		})
	}
	return &hu, nil
}

// GetBySlug retorna una HU con sus escenarios.
func (s *Service) GetBySlug(ctx context.Context, slug string) (*Issue, error) {
	row, err := s.q().GetIssueBySlug(ctx, slug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user_story: %w", err)
	}
	hu := toIssue(row)
	if hu.Scenarios, err = s.listScenarios(ctx, hu.ID); err != nil {
		return nil, err
	}
	return &hu, nil
}

// GetByID retorna una HU por ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Issue, error) {
	row, err := s.q().GetIssueByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user_story: %w", err)
	}
	hu := toIssue(row)
	if hu.Scenarios, err = s.listScenarios(ctx, hu.ID); err != nil {
		return nil, err
	}
	return &hu, nil
}

// List retorna HUs según filtro, con sus escenarios.
func (s *Service) List(ctx context.Context, filter UserStoryFilter) ([]Issue, error) {
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}

	rows, err := s.q().ListIssues(ctx, issuedb.ListIssuesParams{
		Limit:    int32(filter.Limit),
		Offset:   int32(filter.Offset),
		Status:   optStr(filter.Status),
		Priority: optStr(filter.Priority),
		ReqSlug:  optStr(filter.ReqSlug),
	})
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}

	out := make([]Issue, len(rows))
	ids := make([]uuid.UUID, len(rows))
	for i, row := range rows {
		out[i] = toIssue(row)
		ids[i] = out[i].ID
	}

	byID, err := s.listScenariosByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Scenarios = byID[out[i].ID]
	}
	return out, nil
}

// Update actualiza campos de una HU.
func (s *Service) Update(ctx context.Context, slug string, title *string, description *string, status *string, priority *string) (*Issue, error) {
	existing, err := s.GetBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	newTitle := existing.Title
	if title != nil {
		newTitle = *title
	}
	newDesc := existing.Description
	if description != nil {
		if *description == "" {
			newDesc = nil
		} else {
			newDesc = description
		}
	}
	newStatus := existing.Status
	if status != nil {
		if !validStatuses[*status] {
			return nil, ErrInvalidStatus
		}
		newStatus = *status
	}
	newPriority := existing.Priority
	if priority != nil {
		if !validPriorities[*priority] {
			return nil, ErrInvalidPriority
		}
		newPriority = *priority
	}

	row, err := s.q().UpdateIssue(ctx, issuedb.UpdateIssueParams{
		Slug:        slug,
		Title:       newTitle,
		Description: newDesc,
		Status:      newStatus,
		Priority:    newPriority,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update user_story: %w", err)
	}

	updated := toIssue(row)
	if updated.Scenarios, err = s.listScenarios(ctx, updated.ID); err != nil {
		return nil, err
	}

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorType:  audit.ActorSystem,
			Action:     "user_story.updated",
			EntityType: "user_story",
			EntityID:   &updated.ID,
			OldValues:  map[string]any{"title": existing.Title, "status": existing.Status},
			NewValues:  map[string]any{"title": newTitle, "status": newStatus},
		})
	}
	return &updated, nil
}

// Delete elimina una HU.
func (s *Service) Delete(ctx context.Context, slug string) error {
	n, err := s.q().DeleteIssue(ctx, slug)
	if err != nil {
		return fmt.Errorf("delete user_story: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// AddScenario agrega un escenario a una HU existente.
func (s *Service) AddScenario(ctx context.Context, huSlug string, sc Scenario) (*Scenario, error) {
	if err := validateScenario(sc); err != nil {
		return nil, err
	}
	hu, err := s.q().GetIssueBySlug(ctx, huSlug)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user_story: %w", err)
	}

	maxPos, err := s.q().MaxScenarioPosition(ctx, hu.ID)
	if err != nil {
		return nil, fmt.Errorf("max position: %w", err)
	}

	row, err := s.q().InsertScenario(ctx, issuedb.InsertScenarioParams{
		IssueID:   hu.ID,
		ProjectID: hu.ProjectID,
		Feature:   sc.Feature,
		Scenario:  sc.Scenario,
		Given:     sc.Given,
		WhenText:  sc.When,
		ThenRows:  sc.Then,
		Position:  maxPos + 1,
	})
	if err != nil {
		return nil, fmt.Errorf("insert scenario: %w", err)
	}
	inserted := toScenario(row)
	return &inserted, nil
}

// RemoveScenario elimina un escenario por ID.
func (s *Service) RemoveScenario(ctx context.Context, scenarioID uuid.UUID) error {
	n, err := s.q().DeleteScenario(ctx, scenarioID)
	if err != nil {
		return fmt.Errorf("delete scenario: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) listScenarios(ctx context.Context, issueID uuid.UUID) ([]Scenario, error) {
	rows, err := s.q().ListScenariosByIssue(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("list scenarios: %w", err)
	}
	return mapScenarios(rows), nil
}

func (s *Service) listScenariosByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]Scenario, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := s.q().ListScenariosByIssueIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list scenarios by ids: %w", err)
	}
	out := map[uuid.UUID][]Scenario{}
	for _, sc := range mapScenarios(rows) {
		out[sc.HuID] = append(out[sc.HuID], sc)
	}
	return out, nil
}

func toIssue(r issuedb.Issue) Issue {
	pid := r.ProjectID
	return Issue{
		ID:          r.ID,
		Slug:        r.Slug,
		Title:       r.Title,
		Description: r.Description,
		Status:      r.Status,
		Priority:    r.Priority,
		ReqID:       r.ReqID,
		ProjectID:   &pid,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func toScenario(r issuedb.IssueGherkinScenario) Scenario {
	return Scenario{
		ID:        r.ID,
		HuID:      r.IssueID,
		Feature:   r.Feature,
		Scenario:  r.Scenario,
		Given:     r.Given,
		When:      r.WhenText,
		Then:      r.ThenRows,
		Position:  int(r.Position),
		CreatedAt: r.CreatedAt,
	}
}

func mapScenarios(rows []issuedb.IssueGherkinScenario) []Scenario {
	out := make([]Scenario, len(rows))
	for i, r := range rows {
		out[i] = toScenario(r)
	}
	return out
}

func optStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func validateScenarios(scenarios []Scenario) error {
	for i, sc := range scenarios {
		if err := validateScenario(sc); err != nil {
			return fmt.Errorf("scenario %d: %w", i, err)
		}
	}
	return nil
}

func validateScenario(sc Scenario) error {
	if sc.Feature == "" {
		return fmt.Errorf("%w: feature required", ErrScenarioInvalid)
	}
	if sc.Scenario == "" {
		return fmt.Errorf("%w: scenario required", ErrScenarioInvalid)
	}
	if len(sc.Given) == 0 {
		return fmt.Errorf("%w: given required", ErrScenarioInvalid)
	}
	if sc.When == "" {
		return fmt.Errorf("%w: when required", ErrScenarioInvalid)
	}
	if len(sc.Then) == 0 {
		return fmt.Errorf("%w: then required", ErrScenarioInvalid)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation
}
