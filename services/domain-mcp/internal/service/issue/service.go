// Package userstory — issue-04.2 user stories with Gherkin scenarios.
//
// Una HU pertenece a un REQ (requirements) y tiene 1..N issue_gherkin_scenarios
// con campos estructurados (feature, scenario, given[], when, then[]).
package issue

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
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

	var reqID uuid.UUID
	var projectID *uuid.UUID // heredado del requirement padre (scoping por proyecto)
	err := s.Pool.QueryRow(ctx, `SELECT id, project_id FROM sdd_requirements WHERE slug = $1`, reqSlug).Scan(&reqID, &projectID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReqNotFound
		}
		return nil, fmt.Errorf("find req: %w", err)
	}



	if projectID == nil || *projectID == uuid.Nil {
		return nil, ErrProjectIDRequired
	}

	var desc *string
	if description != "" {
		desc = &description
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var hu Issue
	err = tx.QueryRow(ctx,
		`INSERT INTO issues (slug, title, description, status, priority, req_id, project_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, slug, title, description, status, priority, req_id, project_id, created_at, updated_at`,
		slug, title, desc, status, priority, reqID, projectID,
	).Scan(&hu.ID, &hu.Slug, &hu.Title, &hu.Description, &hu.Status, &hu.Priority, &hu.ReqID, &hu.ProjectID, &hu.CreatedAt, &hu.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrSlugTaken
		}
		return nil, fmt.Errorf("insert user_story: %w", err)
	}

	hu.Scenarios, err = insertScenariosTx(ctx, tx, hu.ID, projectID, scenarios)
	if err != nil {
		return nil, err
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
			NewValues:  map[string]any{"slug": slug, "title": title, "req_id": reqID.String()},
		})
	}
	return &hu, nil
}

// GetBySlug retorna una HU con sus escenarios.
func (s *Service) GetBySlug(ctx context.Context, slug string) (*Issue, error) {
	hu, err := s.getBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	scenarios, err := s.listScenarios(ctx, hu.ID)
	if err != nil {
		return nil, err
	}
	hu.Scenarios = scenarios
	return hu, nil
}

// GetByID retorna una HU por ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Issue, error) {
	var hu Issue
	err := s.Pool.QueryRow(ctx,
		`SELECT id, slug, title, description, status, priority, req_id, created_at, updated_at
		 FROM issues WHERE id = $1`, id,
	).Scan(&hu.ID, &hu.Slug, &hu.Title, &hu.Description, &hu.Status, &hu.Priority, &hu.ReqID, &hu.CreatedAt, &hu.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user_story: %w", err)
	}
	scenarios, err := s.listScenarios(ctx, hu.ID)
	if err != nil {
		return nil, err
	}
	hu.Scenarios = scenarios
	return &hu, nil
}

// List retorna HUs según filtro.
func (s *Service) List(ctx context.Context, filter UserStoryFilter) ([]Issue, error) {
	where := []string{"1=1"}
	args := []any{}
	idx := 1

	if filter.Status != "" {
		where = append(where, fmt.Sprintf("us.status = $%d", idx))
		args = append(args, filter.Status)
		idx++
	}
	if filter.Priority != "" {
		where = append(where, fmt.Sprintf("us.priority = $%d", idx))
		args = append(args, filter.Priority)
		idx++
	}
	if filter.ReqSlug != "" {
		where = append(where, fmt.Sprintf("r.slug = $%d", idx))
		args = append(args, filter.ReqSlug)
		idx++
	}

	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}

	q := fmt.Sprintf(`SELECT us.id, us.slug, us.title, us.description, us.status, us.priority, us.req_id, us.created_at, us.updated_at
		 FROM issues us
		 LEFT JOIN sdd_requirements r ON r.id = us.req_id
		 WHERE %s ORDER BY us.slug LIMIT $%d OFFSET $%d`,
		strings.Join(where, " AND "), idx, idx+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := s.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}
	defer rows.Close()

	var out []Issue
	for rows.Next() {
		var hu Issue
		if err := rows.Scan(&hu.ID, &hu.Slug, &hu.Title, &hu.Description, &hu.Status, &hu.Priority, &hu.ReqID, &hu.CreatedAt, &hu.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan user_story: %w", err)
		}
		out = append(out, hu)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}


	ids := make([]uuid.UUID, len(out))
	for i, hu := range out {
		ids[i] = hu.ID
	}
	scenarios, err := s.listScenariosByHuIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Scenarios = scenarios[out[i].ID]
	}
	return out, nil
}

// Update actualiza campos de una HU.
func (s *Service) Update(ctx context.Context, slug string, title *string, description *string, status *string, priority *string) (*Issue, error) {
	existing, err := s.getBySlug(ctx, slug)
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

	var updated Issue
	err = s.Pool.QueryRow(ctx,
		`UPDATE issues SET title = $2, description = $3, status = $4, priority = $5, updated_at = NOW()
		 WHERE slug = $1
		 RETURNING id, slug, title, description, status, priority, req_id, created_at, updated_at`,
		slug, newTitle, newDesc, newStatus, newPriority,
	).Scan(&updated.ID, &updated.Slug, &updated.Title, &updated.Description, &updated.Status, &updated.Priority, &updated.ReqID, &updated.CreatedAt, &updated.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("update user_story: %w", err)
	}

	scenarios, err := s.listScenarios(ctx, updated.ID)
	if err != nil {
		return nil, err
	}
	updated.Scenarios = scenarios

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

// Delete elimina una HU (soft-delete via status archived, o hard delete).
func (s *Service) Delete(ctx context.Context, slug string) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM issues WHERE slug = $1`, slug)
	if err != nil {
		return fmt.Errorf("delete user_story: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AddScenario agrega un escenario a una HU existente.
func (s *Service) AddScenario(ctx context.Context, huSlug string, sc Scenario) (*Scenario, error) {
	hu, err := s.getBySlug(ctx, huSlug)
	if err != nil {
		return nil, err
	}
	if err := validateScenario(sc); err != nil {
		return nil, err
	}


	var maxPos int
	_ = s.Pool.QueryRow(ctx, `SELECT COALESCE(MAX(position), -1) FROM issue_gherkin_scenarios WHERE issue_id = $1`, hu.ID).Scan(&maxPos)
	sc.Position = maxPos + 1

	var inserted Scenario
	err = s.Pool.QueryRow(ctx,
		`INSERT INTO issue_gherkin_scenarios (issue_id, feature, scenario, given, when_text, then_rows, position)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, issue_id, feature, scenario, given, when_text, then_rows, position, created_at`,
		hu.ID, sc.Feature, sc.Scenario, sc.Given, sc.When, sc.Then, sc.Position,
	).Scan(&inserted.ID, &inserted.HuID, &inserted.Feature, &inserted.Scenario, &inserted.Given, &inserted.When, &inserted.Then, &inserted.Position, &inserted.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert scenario: %w", err)
	}
	return &inserted, nil
}

// RemoveScenario elimina un escenario por ID.
func (s *Service) RemoveScenario(ctx context.Context, scenarioID uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `DELETE FROM issue_gherkin_scenarios WHERE id = $1`, scenarioID)
	if err != nil {
		return fmt.Errorf("delete scenario: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}



func (s *Service) getBySlug(ctx context.Context, slug string) (*Issue, error) {
	var hu Issue
	err := s.Pool.QueryRow(ctx,
		`SELECT id, slug, title, description, status, priority, req_id, created_at, updated_at
		 FROM issues WHERE slug = $1`, slug,
	).Scan(&hu.ID, &hu.Slug, &hu.Title, &hu.Description, &hu.Status, &hu.Priority, &hu.ReqID, &hu.CreatedAt, &hu.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get user_story: %w", err)
	}
	return &hu, nil
}

func (s *Service) listScenarios(ctx context.Context, issueID uuid.UUID) ([]Scenario, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, issue_id, feature, scenario, given, when_text, then_rows, position, created_at
		 FROM issue_gherkin_scenarios WHERE issue_id = $1 ORDER BY position`, issueID)
	if err != nil {
		return nil, fmt.Errorf("list scenarios: %w", err)
	}
	defer rows.Close()
	return scanScenarios(rows)
}

func (s *Service) listScenariosByHuIDs(ctx context.Context, huIDs []uuid.UUID) (map[uuid.UUID][]Scenario, error) {
	if len(huIDs) == 0 {
		return nil, nil
	}
	rows, err := s.Pool.Query(ctx,
		`SELECT id, issue_id, feature, scenario, given, when_text, then_rows, position, created_at
		 FROM issue_gherkin_scenarios WHERE issue_id = ANY($1) ORDER BY issue_id, position`, huIDs)
	if err != nil {
		return nil, fmt.Errorf("list scenarios by ids: %w", err)
	}
	defer rows.Close()
	all, err := scanScenarios(rows)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID][]Scenario{}
	for _, sc := range all {
		out[sc.HuID] = append(out[sc.HuID], sc)
	}
	return out, nil
}

func scanScenarios(rows pgx.Rows) ([]Scenario, error) {
	var out []Scenario
	for rows.Next() {
		var sc Scenario
		if err := rows.Scan(&sc.ID, &sc.HuID, &sc.Feature, &sc.Scenario, &sc.Given, &sc.When, &sc.Then, &sc.Position, &sc.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan scenario: %w", err)
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

func insertScenariosTx(ctx context.Context, tx pgx.Tx, issueID uuid.UUID, projectID *uuid.UUID, scenarios []Scenario) ([]Scenario, error) {
	if len(scenarios) == 0 {
		return nil, nil
	}
	out := make([]Scenario, len(scenarios))
	for i, sc := range scenarios {
		sc.HuID = issueID
		sc.Position = i
		err := tx.QueryRow(ctx,
			`INSERT INTO issue_gherkin_scenarios (issue_id, project_id, feature, scenario, given, when_text, then_rows, position)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			 RETURNING id, issue_id, feature, scenario, given, when_text, then_rows, position, created_at`,
			issueID, projectID, sc.Feature, sc.Scenario, sc.Given, sc.When, sc.Then, i,
		).Scan(&out[i].ID, &out[i].HuID, &out[i].Feature, &out[i].Scenario, &out[i].Given, &out[i].When, &out[i].Then, &out[i].Position, &out[i].CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("insert scenario %d: %w", i, err)
		}
	}
	return out, nil
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
