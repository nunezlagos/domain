// Package task — issue-04.4 tasks with status tracking, verification and sabotage.
package task

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
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
	ErrNotFound        = errors.New("task not found")
	ErrInvalidStatus   = errors.New("invalid status")
	ErrInvalidTransition = errors.New("invalid status transition")
	ErrNotCompleted    = errors.New("task not completed")
	ErrHUNotFound      = errors.New("user story not found")
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
	ID         uuid.UUID  `json:"id"`
	TaskID     uuid.UUID  `json:"task_id"`
	Result     string     `json:"result"`
	Evidence   *string    `json:"evidence,omitempty"`
	Notes      *string    `json:"notes,omitempty"`
	VerifiedAt time.Time  `json:"verified_at"`
	VerifiedBy *string    `json:"verified_by,omitempty"`
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
	defer tx.Rollback(ctx)

	// Compute next position per section
	posMap := map[string]int{}
	for _, in := range inputs {
		posMap[in.Section]++
	}
	// Reset to actual next position from DB
	for section := range posMap {
		var maxPos int
		_ = tx.QueryRow(ctx,
			`SELECT COALESCE(MAX(position), 0) FROM tasks WHERE issue_id = $1 AND section = $2`,
			issueID, section,
		).Scan(&maxPos)
		posMap[section] = maxPos + 1
	}

	var out []Task
	for _, in := range inputs {
		pos := posMap[in.Section]
		posMap[in.Section]++

		var t Task
		err := tx.QueryRow(ctx,
			`INSERT INTO tasks (issue_id, section, description, position, status)
			 VALUES ($1, $2, $3, $4, 'pending')
			 RETURNING id, issue_id, section, description, status, position, started_at, completed_at, completed_by, created_at, updated_at`,
			issueID, in.Section, in.Description, pos,
		).Scan(&t.ID, &t.HuID, &t.Section, &t.Description, &t.Status, &t.Position, &t.StartedAt, &t.CompletedAt, &t.CompletedBy, &t.CreatedAt, &t.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("insert task: %w", err)
		}
		out = append(out, t)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}
	return out, nil
}

// ListTasks returns tasks for a HU ordered by section, position.
func (s *Service) ListTasks(ctx context.Context, issueID uuid.UUID) ([]Task, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, issue_id, section, description, status, position, started_at, completed_at, completed_by, created_at, updated_at
		 FROM tasks WHERE issue_id = $1 ORDER BY section, position`, issueID)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()
	return scanTasks(rows)
}

// GetTask returns a single task with its verification and sabotages.
func (s *Service) GetTask(ctx context.Context, taskID uuid.UUID) (*Task, error) {
	var t Task
	err := s.Pool.QueryRow(ctx,
		`SELECT id, issue_id, section, description, status, position, started_at, completed_at, completed_by, created_at, updated_at
		 FROM tasks WHERE id = $1`, taskID,
	).Scan(&t.ID, &t.HuID, &t.Section, &t.Description, &t.Status, &t.Position, &t.StartedAt, &t.CompletedAt, &t.CompletedBy, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}

	// Load verification
	v, err := s.getVerification(ctx, taskID)
	if err != nil {
		return nil, err
	}
	t.Verification = v

	// Load sabotages
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

	var current Task
	err := s.Pool.QueryRow(ctx,
		`SELECT id, issue_id, section, description, status, position, started_at, completed_at, completed_by, created_at, updated_at
		 FROM tasks WHERE id = $1`, taskID,
	).Scan(&current.ID, &current.HuID, &current.Section, &current.Description, &current.Status, &current.Position, &current.StartedAt, &current.CompletedAt, &current.CompletedBy, &current.CreatedAt, &current.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}

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
	var startedAt, completedAt *time.Time
	var cb *string
	if newStatus == StatusInProgress {
		startedAt = &now
	}
	if newStatus == StatusCompleted {
		completedAt = &now
		if completedBy != "" {
			cb = &completedBy
		}
	}

	var updated Task
	err = s.Pool.QueryRow(ctx,
		`UPDATE tasks
		 SET status = $2, started_at = COALESCE($3, started_at), completed_at = $4, completed_by = COALESCE($5, completed_by), updated_at = NOW()
		 WHERE id = $1
		 RETURNING id, issue_id, section, description, status, position, started_at, completed_at, completed_by, created_at, updated_at`,
		taskID, newStatus, startedAt, completedAt, cb,
	).Scan(&updated.ID, &updated.HuID, &updated.Section, &updated.Description, &updated.Status, &updated.Position, &updated.StartedAt, &updated.CompletedAt, &updated.CompletedBy, &updated.CreatedAt, &updated.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update status: %w", err)
	}

	if s.Audit != nil {
		_ = s.Audit.Record(ctx, audit.Event{
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
	var r ProgressReport
	err := s.Pool.QueryRow(ctx,
		`SELECT $1::uuid AS issue_id,
		        COUNT(*) AS total,
		        COUNT(*) FILTER (WHERE status = 'completed') AS completed,
		        ROUND(100.0 * COUNT(*) FILTER (WHERE status = 'completed') / GREATEST(COUNT(*), 1), 1) AS pct
		 FROM tasks WHERE issue_id = $1`, issueID,
	).Scan(&r.HuID, &r.Total, &r.Completed, &r.ProgressPct)
	if err != nil {
		return nil, fmt.Errorf("get progress: %w", err)
	}
	return &r, nil
}

// CreateVerification records a verification result for a completed task.
func (s *Service) CreateVerification(ctx context.Context, taskID uuid.UUID, result, evidence, notes, verifiedBy string) (*VerificationResult, error) {
	if !validVerifResults[result] {
		return nil, fmt.Errorf("invalid verification result: %s", result)
	}

	// Verify task is completed
	var taskStatus string
	err := s.Pool.QueryRow(ctx, `SELECT status FROM tasks WHERE id = $1`, taskID).Scan(&taskStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("check task: %w", err)
	}
	if taskStatus != StatusCompleted {
		return nil, ErrNotCompleted
	}

	var v VerificationResult
	err = s.Pool.QueryRow(ctx,
		`INSERT INTO verification_results (task_id, result, evidence, notes, verified_by)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, task_id, result, evidence, notes, verified_at, verified_by`,
		taskID, result, nullStr(evidence), nullStr(notes), nullStr(verifiedBy),
	).Scan(&v.ID, &v.TaskID, &v.Result, &v.Evidence, &v.Notes, &v.VerifiedAt, &v.VerifiedBy)
	if err != nil {
		return nil, fmt.Errorf("insert verification: %w", err)
	}
	return &v, nil
}

// CreateSabotage records a sabotage action.
func (s *Service) CreateSabotage(ctx context.Context, taskID uuid.UUID, action, expectedFailure, actualResult string, restored bool) (*SabotageRecord, error) {
	var exists bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tasks WHERE id = $1)`, taskID).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("check task: %w", err)
	}
	if !exists {
		return nil, ErrNotFound
	}

	var rec SabotageRecord
	err = s.Pool.QueryRow(ctx,
		`INSERT INTO sabotage_records (task_id, action, expected_failure, actual_result, restored)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, task_id, action, expected_failure, actual_result, restored, performed_at`,
		taskID, action, nullStr(expectedFailure), nullStr(actualResult), restored,
	).Scan(&rec.ID, &rec.TaskID, &rec.Action, &rec.ExpectedFailure, &rec.ActualResult, &rec.Restored, &rec.PerformedAt)
	if err != nil {
		return nil, fmt.Errorf("insert sabotage: %w", err)
	}
	return &rec, nil
}

// ListSabotages returns all sabotage records for a task.
func (s *Service) ListSabotages(ctx context.Context, taskID uuid.UUID) ([]SabotageRecord, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, task_id, action, expected_failure, actual_result, restored, performed_at
		 FROM sabotage_records WHERE task_id = $1 ORDER BY performed_at`, taskID)
	if err != nil {
		return nil, fmt.Errorf("list sabotages: %w", err)
	}
	defer rows.Close()
	return scanSabotages(rows)
}

// --- internal ---

func (s *Service) requireHU(ctx context.Context, issueID uuid.UUID) error {
	var exists bool
	err := s.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM issues WHERE id = $1)`, issueID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check hu: %w", err)
	}
	if !exists {
		return ErrHUNotFound
	}
	return nil
}

func (s *Service) getVerification(ctx context.Context, taskID uuid.UUID) (*VerificationResult, error) {
	var v VerificationResult
	err := s.Pool.QueryRow(ctx,
		`SELECT id, task_id, result, evidence, notes, verified_at, verified_by
		 FROM verification_results WHERE task_id = $1 ORDER BY verified_at DESC LIMIT 1`, taskID,
	).Scan(&v.ID, &v.TaskID, &v.Result, &v.Evidence, &v.Notes, &v.VerifiedAt, &v.VerifiedBy)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get verification: %w", err)
	}
	return &v, nil
}

func nullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func scanTasks(rows pgx.Rows) ([]Task, error) {
	defer rows.Close()
	var out []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.HuID, &t.Section, &t.Description, &t.Status, &t.Position, &t.StartedAt, &t.CompletedAt, &t.CompletedBy, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func scanSabotages(rows pgx.Rows) ([]SabotageRecord, error) {
	defer rows.Close()
	var out []SabotageRecord
	for rows.Next() {
		var s SabotageRecord
		if err := rows.Scan(&s.ID, &s.TaskID, &s.Action, &s.ExpectedFailure, &s.ActualResult, &s.Restored, &s.PerformedAt); err != nil {
			return nil, fmt.Errorf("scan sabotage: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
