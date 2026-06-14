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
	"github.com/jackc/pgx/v5/pgxpool"
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
}

// Execute corre el skill. En sync bloquea y retorna la execution completa;
// en async crea la fila pending, lanza el worker y retorna inmediatamente.
func (s *ExecutionService) Execute(ctx context.Context, in ExecuteInput) (*Execution, error) {
	sk, err := s.Skills.GetByID(ctx, in.SkillID)
	if err != nil {
		return nil, err
	}
	if sk.OrganizationID != in.OrganizationID {
		return nil, ErrNotFound
	}
	if in.Parameters == nil {
		in.Parameters = map[string]any{}
	}
	if in.Mode == "" {
		in.Mode = "sync"
	}

	// Validación de parámetros contra input_schema (issue-05.6 contract)
	schemaJSON, _ := json.Marshal(sk.InputSchema)
	payloadJSON, _ := json.Marshal(in.Parameters)
	if res := ValidatePayload(schemaJSON, payloadJSON); !res.Valid {
		var msgs []string
		for _, ve := range res.Errors {
			msgs = append(msgs, ve.Field+": "+ve.Reason)
		}
		return nil, fmt.Errorf("%w: %s", ErrInvalidParams, strings.Join(msgs, "; "))
	}

	// Resolución de versión: pinned (Effective) tiene precedencia sobre
	// el content actual del skill.
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
		// Worker en background actualiza la fila al terminar.
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
	var e Execution
	var paramsRaw []byte
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO skill_executions
			(organization_id, skill_id, version_used, mode, status, parameters, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		RETURNING id, skill_id, version_used, mode, status, parameters, output, error,
		          execution_time_ms, started_at, completed_at, created_at`,
		in.OrganizationID, sk.ID, versionUsed, in.Mode, status, scrubbed,
	).Scan(&e.ID, &e.SkillID, &e.VersionUsed, &e.Mode, &e.Status, &paramsRaw,
		&e.Output, &e.Error, &e.ExecutionTimeMs, &e.StartedAt, &e.CompletedAt, &e.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert execution: %w", err)
	}
	_ = json.Unmarshal(paramsRaw, &e.Parameters)
	return &e, nil
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

	_, _ = s.Pool.Exec(ctx,
		`UPDATE skill_executions SET status = 'running' WHERE id = $1 AND status = 'pending'`, execID)

	start := time.Now()
	output, err := s.Runner.Execute(runCtx, sk, in.Parameters)
	elapsed := int(time.Since(start).Milliseconds())

	if err != nil {
		msg := err.Error()
		_, _ = s.Pool.Exec(ctx, `
			UPDATE skill_executions SET status = 'failed', error = $2,
			  execution_time_ms = $3, completed_at = NOW() WHERE id = $1`,
			execID, msg, elapsed)
		return
	}
	_, _ = s.Pool.Exec(ctx, `
		UPDATE skill_executions SET status = 'completed', output = $2,
		  execution_time_ms = $3, completed_at = NOW() WHERE id = $1`,
		execID, output, elapsed)
}

// Get retorna una execution con guard de org (anti-enumeration).
func (s *ExecutionService) Get(ctx context.Context, orgID, id uuid.UUID) (*Execution, error) {
	var e Execution
	var paramsRaw []byte
	err := s.Pool.QueryRow(ctx, `
		SELECT id, skill_id, version_used, mode, status, parameters, output, error,
		       execution_time_ms, started_at, completed_at, created_at
		FROM skill_executions WHERE id = $1 AND organization_id = $2`,
		id, orgID,
	).Scan(&e.ID, &e.SkillID, &e.VersionUsed, &e.Mode, &e.Status, &paramsRaw,
		&e.Output, &e.Error, &e.ExecutionTimeMs, &e.StartedAt, &e.CompletedAt, &e.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrExecutionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get execution: %w", err)
	}
	_ = json.Unmarshal(paramsRaw, &e.Parameters)
	return &e, nil
}
