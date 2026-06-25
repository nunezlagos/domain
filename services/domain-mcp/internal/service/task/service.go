// Package task — issue-04.4 tasks with status tracking, verification and sabotage.
//
//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
package task

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/service/task/taskdb"
	"nunezlagos/domain/internal/store/txctx"
)

const (
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"

	VerifPass    = "pass"
	VerifFail    = "fail"
	VerifBlocked = "blocked"
)

var (
	ErrNotFound          = errors.New("task not found")
	ErrInvalidStatus     = errors.New("invalid status")
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrNotCompleted      = errors.New("task not completed")
	ErrHUNotFound        = errors.New("user story not found")
)

var validStatuses = map[string]bool{StatusPending: true, StatusInProgress: true, StatusCompleted: true}
var allowedTransitions = map[string][]string{
	StatusPending:    {StatusInProgress},
	StatusInProgress: {StatusCompleted},
	StatusCompleted:  {},
}

var validVerifResults = map[string]bool{VerifPass: true, VerifFail: true, VerifBlocked: true}

type Task struct {
	ID          uuid.UUID  `json:"id"`
	HuID        uuid.UUID  `json:"issue_id"`
	Section     string     `json:"section"`
	Description string     `json:"description"`
	Status      string     `json:"status"`
	Position    int        `json:"position"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CompletedBy *string    `json:"completed_by,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	Verification *VerificationResult `json:"verification,omitempty"`
	Sabotages    []SabotageRecord    `json:"sabotages,omitempty"`
}

type VerificationResult struct {
	ID         uuid.UUID `json:"id"`
	TaskID     uuid.UUID `json:"task_id"`
	Result     string    `json:"result"`
	Evidence   *string   `json:"evidence,omitempty"`
	Notes      *string   `json:"notes,omitempty"`
	VerifiedAt time.Time `json:"verified_at"`
	VerifiedBy *string   `json:"verified_by,omitempty"`
}

type SabotageRecord struct {
	ID              uuid.UUID `json:"id"`
	TaskID          uuid.UUID `json:"task_id"`
	Action          string    `json:"action"`
	ExpectedFailure *string   `json:"expected_failure,omitempty"`
	ActualResult    *string   `json:"actual_result,omitempty"`
	Restored        bool      `json:"restored"`
	PerformedAt     time.Time `json:"performed_at"`
}

type ProgressReport struct {
	HuID        uuid.UUID `json:"issue_id"`
	Total       int       `json:"total"`
	Completed   int       `json:"completed"`
	ProgressPct float64   `json:"progress_pct"`
}

type CreateTaskInput struct {
	Section     string `json:"section"`
	Description string `json:"description"`
}

type Service struct {
	Pool  *pgxpool.Pool
	Audit *audit.PGRecorder
}

func (s *Service) q(ctx context.Context) *taskdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return taskdb.New(tx)
	}
	return taskdb.New(s.Pool)
}

// CreateTasks batch-creates tasks with auto-assigned position per section.
func (s *Service) CreateTasks(ctx context.Context, issueID uuid.UUID, inputs []CreateTaskInput) ([]Task, error) {
	if err := s.requireHU(ctx, issueID); err != nil {
		return nil, err
	}
	if len(inputs) == 0 {
		return nil, fmt.Errorf("no tasks provided")
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := taskdb.New(tx)

	projectID, err := q.GetIssueProjectID(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("get issue project_id: %w", err)
	}

	posMap := map[string]int{}
	for _, in := range inputs {
		posMap[in.Section]++
	}

	for section := range posMap {
		maxPos, err := q.MaxTaskPosition(ctx, taskdb.MaxTaskPositionParams{IssueID: issueID, Section: section})
		if err != nil {
			return nil, fmt.Errorf("max position: %w", err)
		}
		posMap[section] = int(maxPos) + 1
	}

	var out []Task
	for _, in := range inputs {
		pos := posMap[in.Section]
		posMap[in.Section]++

		row, err := q.InsertTask(ctx, taskdb.InsertTaskParams{
			IssueID:     issueID,
			ProjectID:   projectID,
			Section:     in.Section,
			Description: in.Description,
			Position:    int32(pos),
		})
		if err != nil {
			return nil, fmt.Errorf("insert task: %w", err)
		}
		out = append(out, toTask(row))
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return out, nil
}

// ListTasks returns tasks for a HU ordered by section, position.
func (s *Service) ListTasks(ctx context.Context, issueID uuid.UUID) ([]Task, error) {
	rows, err := s.q(ctx).ListTasksByIssue(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	out := make([]Task, len(rows))
	for i, r := range rows {
		out[i] = toTask(r)
	}
	return out, nil
}

// GetTask returns a single task with its verification and sabotages.
func (s *Service) GetTask(ctx context.Context, taskID uuid.UUID) (*Task, error) {
	row, err := s.q(ctx).GetTaskByID(ctx, taskID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	t := toTask(row)

	v, err := s.getVerification(ctx, taskID)
	if err != nil {
		return nil, err
	}
	t.Verification = v

	sabs, err := s.ListSabotages(ctx, taskID)
	if err != nil {
		return nil, err
	}
	t.Sabotages = sabs

	return &t, nil
}

// UpdateTaskStatus transitions status with validation.
func (s *Service) UpdateTaskStatus(ctx context.Context, taskID uuid.UUID, newStatus, completedBy string) (*Task, error) {
	if !validStatuses[newStatus] {
		return nil, ErrInvalidStatus
	}

	row, err := s.q(ctx).GetTaskByID(ctx, taskID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	current := toTask(row)

	allowed, ok := allowedTransitions[current.Status]
	if !ok {
		return nil, ErrInvalidTransition
	}
	valid := false
	for _, a := range allowed {
		if a == newStatus {
			valid = true
			break
		}
	}
	if !valid {
		return nil, fmt.Errorf("%w: %s → %s", ErrInvalidTransition, current.Status, newStatus)
	}

	now := time.Now()
	var startedAt, completedAt pgtype.Timestamptz
	var cb *string
	if newStatus == StatusInProgress {
		startedAt = pgtype.Timestamptz{Time: now, Valid: true}
	}
	if newStatus == StatusCompleted {
		completedAt = pgtype.Timestamptz{Time: now, Valid: true}
		if completedBy != "" {
			cb = &completedBy
		}
	}

	updatedRow, err := s.q(ctx).UpdateTaskStatus(ctx, taskdb.UpdateTaskStatusParams{
		ID:          taskID,
		Status:      newStatus,
		StartedAt:   startedAt,
		CompletedAt: completedAt,
		CompletedBy: cb,
	})
	if err != nil {
		return nil, fmt.Errorf("update status: %w", err)
	}
	updated := toTask(updatedRow)

	if s.Audit != nil {
		audit.RecordOrLog(ctx, s.Audit, audit.Event{
			ActorType:  audit.ActorSystem,
			Action:     "task.status_changed",
			EntityType: "task",
			EntityID:   &updated.ID,
			OldValues:  map[string]any{"status": current.Status},
			NewValues:  map[string]any{"status": newStatus},
		})
	}
	return &updated, nil
}

// GetProgress returns aggregate progress for a HU.
func (s *Service) GetProgress(ctx context.Context, issueID uuid.UUID) (*ProgressReport, error) {
	row, err := s.q(ctx).GetProgress(ctx, issueID)
	if err != nil {
		return nil, fmt.Errorf("get progress: %w", err)
	}
	return &ProgressReport{
		HuID:        row.IssueID,
		Total:       int(row.Total),
		Completed:   int(row.Completed),
		ProgressPct: row.Pct,
	}, nil
}

// CreateVerification records a verification result for a completed task.
func (s *Service) CreateVerification(ctx context.Context, taskID uuid.UUID, result, evidence, notes, verifiedBy string) (*VerificationResult, error) {
	if !validVerifResults[result] {
		return nil, fmt.Errorf("invalid verification result: %s", result)
	}

	taskStatus, err := s.q(ctx).GetTaskStatus(ctx, taskID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("check task: %w", err)
	}
	if taskStatus != StatusCompleted {
		return nil, ErrNotCompleted
	}

	row, err := s.q(ctx).InsertVerification(ctx, taskdb.InsertVerificationParams{
		TaskID:     taskID,
		Result:     result,
		Evidence:   nullStr(evidence),
		Notes:      nullStr(notes),
		VerifiedBy: nullStr(verifiedBy),
	})
	if err != nil {
		return nil, fmt.Errorf("insert verification: %w", err)
	}
	v := toVerification(row)
	return &v, nil
}

// CreateSabotage records a sabotage action.
func (s *Service) CreateSabotage(ctx context.Context, taskID uuid.UUID, action, expectedFailure, actualResult string, restored bool) (*SabotageRecord, error) {
	exists, err := s.q(ctx).TaskExists(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("check task: %w", err)
	}
	if !exists {
		return nil, ErrNotFound
	}

	row, err := s.q(ctx).InsertSabotage(ctx, taskdb.InsertSabotageParams{
		TaskID:          taskID,
		Action:          action,
		ExpectedFailure: nullStr(expectedFailure),
		ActualResult:    nullStr(actualResult),
		Restored:        restored,
	})
	if err != nil {
		return nil, fmt.Errorf("insert sabotage: %w", err)
	}
	rec := toSabotage(row)
	return &rec, nil
}

// ListSabotages returns all sabotage records for a task.
func (s *Service) ListSabotages(ctx context.Context, taskID uuid.UUID) ([]SabotageRecord, error) {
	rows, err := s.q(ctx).ListSabotagesByTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("list sabotages: %w", err)
	}
	out := make([]SabotageRecord, len(rows))
	for i, r := range rows {
		out[i] = toSabotage(r)
	}
	return out, nil
}

func (s *Service) requireHU(ctx context.Context, issueID uuid.UUID) error {
	exists, err := s.q(ctx).IssueExists(ctx, issueID)
	if err != nil {
		return fmt.Errorf("check hu: %w", err)
	}
	if !exists {
		return ErrHUNotFound
	}
	return nil
}

func (s *Service) getVerification(ctx context.Context, taskID uuid.UUID) (*VerificationResult, error) {
	row, err := s.q(ctx).GetLatestVerification(ctx, taskID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get verification: %w", err)
	}
	v := toVerification(row)
	return &v, nil
}

func toTask(r taskdb.IssueTask) Task {
	return Task{
		ID:          r.ID,
		HuID:        r.IssueID,
		Section:     r.Section,
		Description: r.Description,
		Status:      r.Status,
		Position:    int(r.Position),
		StartedAt:   tsPtr(r.StartedAt),
		CompletedAt: tsPtr(r.CompletedAt),
		CompletedBy: r.CompletedBy,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

func toVerification(r taskdb.TddVerificationResult) VerificationResult {
	return VerificationResult{
		ID:         r.ID,
		TaskID:     r.TaskID,
		Result:     r.Result,
		Evidence:   r.Evidence,
		Notes:      r.Notes,
		VerifiedAt: r.VerifiedAt,
		VerifiedBy: r.VerifiedBy,
	}
}

func toSabotage(r taskdb.TddSabotageRecord) SabotageRecord {
	return SabotageRecord{
		ID:              r.ID,
		TaskID:          r.TaskID,
		Action:          r.Action,
		ExpectedFailure: r.ExpectedFailure,
		ActualResult:    r.ActualResult,
		Restored:        r.Restored,
		PerformedAt:     r.PerformedAt,
	}
}

func tsPtr(ts pgtype.Timestamptz) *time.Time {
	if !ts.Valid {
		return nil
	}
	t := ts.Time
	return &t
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
