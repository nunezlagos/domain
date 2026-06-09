// Package lifecycletrack — HU-04.10 lifecycle state tracking unificado.
//
// Provee state machines declarativas por entity_kind con validación de
// transiciones y registro append-only en entity_state_transitions.
//
// La tabla es audit immutable (trigger anti-UPDATE/DELETE). El servicio
// expone Record para registrar transiciones validadas, ListByEntity para
// timeline, y Stuck para detectar entidades atascadas.
//
// Diferencia con paquetes existentes (lifecycle): lifecycle maneja restore
// de soft-delete. Este paquete cubre transitions de status para todas las
// entidades del workflow SDD (intake → req → hu → sync_state → proposal/
// design/task).
package lifecycletrack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidEntity     = errors.New("invalid entity_kind")
	ErrInvalidTransition = errors.New("transition not allowed")
	ErrMissingActor      = errors.New("actor required")
)

const (
	EntityIntake     = "intake"
	EntityREQ        = "req"
	EntityHU         = "hu"
	EntitySyncState  = "sync_state"
	EntityProposal   = "proposal"
	EntityDesign     = "design"
	EntityTask       = "task"

	ActorUser     = "user"
	ActorAgent    = "agent"
	ActorSystem   = "system"
	ActorExternal = "external"
)

// stateMachines define las transiciones permitidas por entity_kind.
// Una transición vacía (slice nil) significa estado terminal.
// El estado "" (vacío) representa creación inicial.
var stateMachines = map[string]map[string][]string{
	EntityHU: {
		"":             {"proposed"},
		"proposed":     {"approved", "rejected"},
		"approved":     {"in_progress", "rejected"},
		"in_progress":  {"done", "blocked"},
		"blocked":      {"in_progress", "rejected"},
		"done":         {"archived"},
		"rejected":     {},
		"archived":     {},
	},
	EntityREQ: {
		"":          {"draft", "active"},
		"draft":     {"active", "archived"},
		"active":    {"completed", "archived"},
		"completed": {"archived"},
		"archived":  {},
	},
	EntityIntake: {
		"":              {"received"},
		"received":      {"classifying", "failed"},
		"classifying":   {"classified", "failed"},
		"classified":    {"deduping"},
		"deduping":      {"structuring"},
		"structuring":   {"pending_review", "failed"},
		"pending_review": {"approved", "rejected"},
		"approved":      {"committed"},
		"rejected":      {},
		"committed":     {},
		"failed":        {"received"},
	},
	EntitySyncState: {
		"":         {"pending"},
		"pending":  {"ok", "failed"},
		"ok":       {"partial", "conflict", "disabled"},
		"partial":  {"ok", "failed"},
		"conflict": {"ok", "disabled"},
		"failed":   {"pending", "disabled"},
		"disabled": {"pending"},
	},
	EntityProposal: {
		"":          {"draft"},
		"draft":     {"approved", "rejected"},
		"approved":  {"archived"},
		"rejected":  {},
		"archived":  {},
	},
	EntityDesign: {
		"":            {"draft"},
		"draft":       {"final"},
		"final":       {"superseded"},
		"superseded":  {},
	},
	EntityTask: {
		"":            {"pending"},
		"pending":     {"in_progress", "blocked"},
		"in_progress": {"completed", "blocked"},
		"blocked":     {"in_progress"},
		"completed":   {},
	},
}

type Actor struct {
	Kind string     // user|agent|system|external
	ID   *uuid.UUID // opcional para system/external
	Name string     // identificación legible (email, "claude-code", "jira-webhook")
}

type Transition struct {
	ID         int64           `json:"id"`
	EntityKind string          `json:"entity_kind"`
	EntityID   uuid.UUID       `json:"entity_id"`
	FromState  *string         `json:"from_state,omitempty"`
	ToState    string          `json:"to_state"`
	ActorKind  string          `json:"actor_kind"`
	ActorID    *uuid.UUID      `json:"actor_id,omitempty"`
	ActorName  *string         `json:"actor_name,omitempty"`
	Reason     *string         `json:"reason,omitempty"`
	Context    json.RawMessage `json:"context,omitempty"`
	TxID       *uuid.UUID      `json:"tx_id,omitempty"`
	OccurredAt time.Time       `json:"occurred_at"`
}

type StuckRow struct {
	EntityKind       string    `json:"entity_kind"`
	EntityID         uuid.UUID `json:"entity_id"`
	CurrentState     string    `json:"current_state"`
	LastTransitionAt time.Time `json:"last_transition_at"`
	HoursInState     float64   `json:"hours_in_state"`
}

type Service struct {
	Pool *pgxpool.Pool
}

// AllowedTransitions devuelve los siguientes estados válidos desde fromState.
func AllowedTransitions(entityKind, fromState string) []string {
	machine, ok := stateMachines[entityKind]
	if !ok {
		return nil
	}
	return machine[fromState]
}

// CanTransition valida si fromState→toState es permitido.
func CanTransition(entityKind, fromState, toState string) bool {
	for _, allowed := range AllowedTransitions(entityKind, fromState) {
		if allowed == toState {
			return true
		}
	}
	return false
}

// Record persiste una transición tras validar contra la state machine.
// fromState vacío representa creación (entity nueva).
func (s *Service) Record(ctx context.Context, entityKind string, entityID uuid.UUID, fromState, toState string, actor Actor, reason string, txContext map[string]any, txID *uuid.UUID) (*Transition, error) {
	if _, ok := stateMachines[entityKind]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrInvalidEntity, entityKind)
	}
	if actor.Kind == "" {
		return nil, ErrMissingActor
	}
	if !CanTransition(entityKind, fromState, toState) {
		return nil, fmt.Errorf("%w: %s → %s (entity=%s)",
			ErrInvalidTransition, fromState, toState, entityKind)
	}

	var fs *string
	if fromState != "" {
		fs = &fromState
	}
	var an *string
	if actor.Name != "" {
		an = &actor.Name
	}
	var r *string
	if reason != "" {
		r = &reason
	}
	var ctxJSON []byte
	if txContext != nil && len(txContext) > 0 {
		ctxJSON, _ = json.Marshal(txContext)
	}

	var t Transition
	err := s.Pool.QueryRow(ctx, `
		INSERT INTO entity_state_transitions
		  (entity_kind, entity_id, from_state, to_state, actor_kind, actor_id, actor_name,
		   reason, context, tx_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, entity_kind, entity_id, from_state, to_state, actor_kind, actor_id,
		          actor_name, reason, context, tx_id, occurred_at`,
		entityKind, entityID, fs, toState, actor.Kind, actor.ID, an,
		r, ctxJSON, txID,
	).Scan(&t.ID, &t.EntityKind, &t.EntityID, &t.FromState, &t.ToState, &t.ActorKind,
		&t.ActorID, &t.ActorName, &t.Reason, &t.Context, &t.TxID, &t.OccurredAt)
	if err != nil {
		return nil, fmt.Errorf("insert transition: %w", err)
	}
	return &t, nil
}

// ListByEntity devuelve el timeline ordenado por occurred_at ASC.
func (s *Service) ListByEntity(ctx context.Context, entityKind string, entityID uuid.UUID, limit int) ([]Transition, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id, entity_kind, entity_id, from_state, to_state, actor_kind, actor_id,
		       actor_name, reason, context, tx_id, occurred_at
		FROM entity_state_transitions
		WHERE entity_kind = $1 AND entity_id = $2
		ORDER BY occurred_at ASC, id ASC
		LIMIT $3`, entityKind, entityID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list transitions: %w", err)
	}
	defer rows.Close()

	var out []Transition
	for rows.Next() {
		var t Transition
		if err := rows.Scan(&t.ID, &t.EntityKind, &t.EntityID, &t.FromState, &t.ToState,
			&t.ActorKind, &t.ActorID, &t.ActorName, &t.Reason, &t.Context,
			&t.TxID, &t.OccurredAt); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// CurrentState devuelve el último estado conocido (last to_state) para una entidad.
func (s *Service) CurrentState(ctx context.Context, entityKind string, entityID uuid.UUID) (string, error) {
	var state string
	err := s.Pool.QueryRow(ctx, `
		SELECT to_state FROM entity_state_transitions
		WHERE entity_kind = $1 AND entity_id = $2
		ORDER BY occurred_at DESC, id DESC LIMIT 1`,
		entityKind, entityID,
	).Scan(&state)
	if err != nil {
		return "", fmt.Errorf("current state: %w", err)
	}
	return state, nil
}

// Stuck devuelve entidades cuya última transición fue hace más de minHours.
// Usa la view v_stuck_entities materializada en migration 60.
func (s *Service) Stuck(ctx context.Context, minHours float64, limit int) ([]StuckRow, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT entity_kind, entity_id, current_state, last_transition_at, hours_in_state
		FROM v_stuck_entities
		WHERE hours_in_state >= $1
		ORDER BY hours_in_state DESC LIMIT $2`, minHours, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("stuck query: %w", err)
	}
	defer rows.Close()

	var out []StuckRow
	for rows.Next() {
		var r StuckRow
		if err := rows.Scan(&r.EntityKind, &r.EntityID, &r.CurrentState,
			&r.LastTransitionAt, &r.HoursInState); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
