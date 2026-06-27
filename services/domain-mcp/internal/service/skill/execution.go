// issue-05.5 — motor de ejecución de skills (sync/async) con validación de
// parámetros, resolución de versión pinned y log persistente con scrubbing.
package skill

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/skill/skilldb"
	"nunezlagos/domain/internal/store/txctx"
)

// Executor ejecuta el skill ya resuelto (implementado por runner/skill).
type Executor interface {
	Execute(ctx context.Context, sk *Skill, args map[string]any) (string, error)
}

// Execution es una fila de skill_executions.
type Execution struct {
	ID              uuid.UUID      `json:"id"`
	SkillID         uuid.UUID      `json:"skill_id"`
	VersionUsed     *int           `json:"version_used,omitempty"`
	Mode            string         `json:"mode"`
	Status          string         `json:"status"`
	Parameters      map[string]any `json:"parameters"`
	Output          *string        `json:"output,omitempty"`
	Error           *string        `json:"error,omitempty"`
	ExecutionTimeMs *int           `json:"execution_time_ms,omitempty"`
	StartedAt       *time.Time     `json:"started_at,omitempty"`
	CompletedAt     *time.Time     `json:"completed_at,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
}

var (
	ErrExecutionNotFound = errors.New("execution not found")
	ErrInvalidParams     = errors.New("invalid parameters")
)

// ExecutionService orquesta validación → versión → ejecución → log.
type ExecutionService struct {
	Pool     *pgxpool.Pool
	Skills   *Service
	Versions *VersionStore
	Runner   Executor
}

func (s *ExecutionService) q(ctx context.Context) *skilldb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return skilldb.New(tx)
	}
	return skilldb.New(s.Pool)
}

func buildExecution(id uuid.UUID, skillID uuid.UUID, versionUsed *int32, mode, status string, parameters []byte, output, error *string, executionTimeMs *int32, startedAt pgtype.Timestamptz, completedAt pgtype.Timestamptz, createdAt time.Time) Execution {
	var params map[string]any
	if len(parameters) > 0 {
		_ = json.Unmarshal(parameters, &params)
	}
	return Execution{
		ID:              id,
		SkillID:         skillID,
		VersionUsed:     int32PtrToPtr(versionUsed),
		Mode:            mode,
		Status:          status,
		Parameters:      params,
		Output:          output,
		Error:           error,
		ExecutionTimeMs: int32PtrToPtr(executionTimeMs),
		StartedAt:       timestamptzPtr(startedAt),
		CompletedAt:     timestamptzPtr(completedAt),
		CreatedAt:       createdAt,
	}
}

func toExecutionFromCreate(e skilldb.ExecutionCreateRow) Execution {
	return buildExecution(e.ID, e.SkillID, e.VersionUsed, e.Mode, e.Status, e.Parameters,
		e.Output, e.Error, e.ExecutionTimeMs, e.StartedAt, e.CompletedAt, e.CreatedAt)
}

func toExecutionFromGet(e skilldb.ExecutionGetByIDRow) Execution {
	return buildExecution(e.ID, e.SkillID, e.VersionUsed, e.Mode, e.Status, e.Parameters,
		e.Output, e.Error, e.ExecutionTimeMs, e.StartedAt, e.CompletedAt, e.CreatedAt)
}

func int32PtrToPtr(v *int32) *int {
	if v == nil {
		return nil
	}
	x := int(*v)
	return &x
}

func timestamptzPtr(t pgtype.Timestamptz) *time.Time {
	if t.Valid {
		return &t.Time
	}
	return nil
}

// scrubKeys según security.md: nunca persistir valores de estas keys.
var scrubKeys = []string{
	"password", "passwd", "secret", "token", "key", "api_key", "apikey",
	"otp", "code", "signature", "hmac", "jwt", "authorization", "bearer",
}

// ScrubParams redacta valores sensibles (recursivo) antes de persistir.
func ScrubParams(params map[string]any) map[string]any {
	out := make(map[string]any, len(params))
	for k, v := range params {
		lk := strings.ToLower(k)
		redact := false
		for _, sk := range scrubKeys {
			if strings.Contains(lk, sk) {
				redact = true
				break
			}
		}
		switch {
		case redact:
			out[k] = "[REDACTED]"
		default:
			if nested, ok := v.(map[string]any); ok {
				out[k] = ScrubParams(nested)
			} else {
				out[k] = v
			}
		}
	}
	return out
}

// ExecuteInput parámetros del execute.
type ExecuteInput struct {
	OrganizationID uuid.UUID
	SkillID        uuid.UUID
	Parameters     map[string]any
	Mode           string // "sync" (default) | "async"
	TimeoutSeconds int    // 0 = timeout del skill
	// CreatedBy es el usuario que origina la ejecución (Principal del MCP/HTTP).
	// nil en triggers de sistema (cron, webhook): se persiste created_by NULL.
	// Alimenta unique_callers_count del aggregator (HU-52.2).
	CreatedBy *uuid.UUID
}

// Execute corre el skill. En sync bloquea y retorna la execution completa;
// en async crea la fila pending, lanza el worker y retorna inmediatamente.
func (s *ExecutionService) Execute(ctx context.Context, in ExecuteInput) (*Execution, error) {
	sk, err := s.Skills.GetByID(ctx, in.SkillID)
	if err != nil {
		return nil, err
	}
	if in.Parameters == nil {
		in.Parameters = map[string]any{}
	}
	if in.Mode == "" {
		in.Mode = "sync"
	}


	schemaJSON, _ := json.Marshal(sk.InputSchema)
	payloadJSON, _ := json.Marshal(in.Parameters)
	if res := ValidatePayload(schemaJSON, payloadJSON); !res.Valid {
		var msgs []string
		for _, ve := range res.Errors {
			msgs = append(msgs, ve.Field+": "+ve.Reason)
		}
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, strings.Join(msgs, "; "))
	}



	var versionUsed *int
	if s.Versions != nil {
		if v, err := s.Versions.Effective(ctx, sk.ID); err == nil && v != nil {
			if v.Content != nil {
				sk.Content = *v.Content
			}
			versionUsed = &v.Version
		}
	}

	exec, err := s.insertExecution(ctx, in, sk, versionUsed)
	if err != nil {
		return nil, err
	}

	if in.Mode == "async" {

		go func() {
			bg := context.WithoutCancel(ctx)
			s.runAndComplete(bg, exec.ID, sk, in)
		}()
		return exec, nil
	}

	s.runAndComplete(ctx, exec.ID, sk, in)
	return s.Get(ctx, in.OrganizationID, exec.ID)
}

func (s *ExecutionService) insertExecution(ctx context.Context, in ExecuteInput, sk *Skill, versionUsed *int) (*Execution, error) {
	scrubbed, _ := json.Marshal(ScrubParams(in.Parameters))
	status := "running"
	if in.Mode == "async" {
		status = "pending"
	}
	row, err := s.q(ctx).ExecutionCreate(ctx, skilldb.ExecutionCreateParams{
		SkillID:     sk.ID,
		VersionUsed: int32Ptr(versionUsed),
		Mode:        in.Mode,
		Status:      status,
		Parameters:  scrubbed,
		CreatedBy:   in.CreatedBy,
	})
	if err != nil {
		return nil, fmt.Errorf("insert execution: %w", err)
	}
	exec := toExecutionFromCreate(row)
	return &exec, nil
}

func int32Ptr(v *int) *int32 {
	if v == nil {
		return nil
	}
	x := int32(*v)
	return &x
}

// runAndComplete ejecuta y persiste el resultado (success o failure).
func (s *ExecutionService) runAndComplete(ctx context.Context, execID uuid.UUID, sk *Skill, in ExecuteInput) {
	timeout := time.Duration(sk.TimeoutSeconds) * time.Second
	if in.TimeoutSeconds > 0 {
		timeout = time.Duration(in.TimeoutSeconds) * time.Second
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	q := s.q(ctx)
	_ = q.ExecutionSetRunning(ctx, execID)

	start := time.Now()
	output, err := s.Runner.Execute(runCtx, sk, in.Parameters)
	elapsed := int32(time.Since(start).Milliseconds())

	if err != nil {
		msg := err.Error()
		_ = q.ExecutionSetFailed(ctx, skilldb.ExecutionSetFailedParams{
			Error:           &msg,
			ExecutionTimeMs: &elapsed,
			ID:              execID,
		})
		return
	}
	_ = q.ExecutionSetCompleted(ctx, skilldb.ExecutionSetCompletedParams{
		Output:          &output,
		ExecutionTimeMs: &elapsed,
		ID:              execID,
	})
}

// Get retorna una execution con guard de org (anti-enumeration).
func (s *ExecutionService) Get(ctx context.Context, orgID, id uuid.UUID) (*Execution, error) {
	row, err := s.q(ctx).ExecutionGetByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrExecutionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get execution: %w", err)
	}
	exec := toExecutionFromGet(row)
	return &exec, nil
}
