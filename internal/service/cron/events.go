// HU-10.3 event-execution — bus de eventos internos para disparar flows/
// agents/skills cuando ocurre un trigger declarativo (no cron, no webhook).
//
// Ejemplo: cuando se crea una observation con tag X, ejecuta el flow Y.
//
// Eventos in-process (Bus) + persistencia para replay (event_log).
package cron

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Event es un evento publicado al bus.
type Event struct {
	ID             uuid.UUID       `json:"id"`
	Type           string          `json:"type"`           // ej: observation.created, run.completed
	OrganizationID *uuid.UUID      `json:"organization_id,omitempty"`
	ProjectID      *uuid.UUID      `json:"project_id,omitempty"`
	Payload        json.RawMessage `json:"payload,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
}

// Handler es la función que procesa un evento.
type Handler func(context.Context, Event) error

// EventBus mantiene subscriptions por type. Async fan-out con error logging.
type EventBus struct {
	Pool   *pgxpool.Pool
	Logger *slog.Logger

	mu       sync.RWMutex
	handlers map[string][]Handler // key: event type (suporta wildcard "*")
}

// NewEventBus inicializa un bus listo para Subscribe + Publish.
func NewEventBus(pool *pgxpool.Pool, logger *slog.Logger) *EventBus {
	if logger == nil {
		logger = slog.Default()
	}
	return &EventBus{
		Pool:     pool,
		Logger:   logger,
		handlers: make(map[string][]Handler),
	}
}

// Subscribe registra un handler para un type. Wildcard "*" recibe todos.
// Convención: "<resource>.<action>" (ej: observation.created, run.failed).
func (b *EventBus) Subscribe(eventType string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], h)
}

// Publish dispara fan-out async + persiste en event_log para audit/replay.
// Best-effort: errores de persistencia se logean pero no bloquean fan-out.
func (b *EventBus) Publish(ctx context.Context, ev Event) error {
	if ev.ID == uuid.Nil {
		ev.ID = uuid.New()
	}
	if ev.CreatedAt.IsZero() {
		ev.CreatedAt = time.Now()
	}

	// Persistir antes del fan-out (durability primero)
	if err := b.persist(ctx, ev); err != nil {
		b.Logger.Warn("event persist failed", slog.String("type", ev.Type), slog.Any("err", err))
	}

	b.mu.RLock()
	handlers := append([]Handler{}, b.handlers[ev.Type]...)
	handlers = append(handlers, b.handlers["*"]...)
	// soporta prefijo: "observation.*" matchea observation.created/updated
	for key, hs := range b.handlers {
		if strings.HasSuffix(key, ".*") {
			prefix := strings.TrimSuffix(key, ".*") + "."
			if strings.HasPrefix(ev.Type, prefix) {
				handlers = append(handlers, hs...)
			}
		}
	}
	b.mu.RUnlock()

	for _, h := range handlers {
		go b.runHandler(ctx, ev, h)
	}
	return nil
}

func (b *EventBus) runHandler(ctx context.Context, ev Event, h Handler) {
	defer func() {
		if r := recover(); r != nil {
			b.Logger.Error("event handler panic",
				slog.String("type", ev.Type),
				slog.Any("panic", r))
		}
	}()
	if err := h(ctx, ev); err != nil {
		b.Logger.Warn("event handler error",
			slog.String("type", ev.Type),
			slog.String("event_id", ev.ID.String()),
			slog.Any("err", err))
	}
}

func (b *EventBus) persist(ctx context.Context, ev Event) error {
	_, err := b.Pool.Exec(ctx, `
		INSERT INTO event_log (id, type, organization_id, project_id, payload, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		ev.ID, ev.Type, ev.OrganizationID, ev.ProjectID, ev.Payload, ev.CreatedAt,
	)
	return err
}

// Replay retorna eventos persistidos para replay (debugging / cold start).
func (b *EventBus) Replay(ctx context.Context, typeFilter string, since time.Time, limit int) ([]Event, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	q := `SELECT id, type, organization_id, project_id, payload, created_at
	      FROM event_log WHERE created_at >= $1`
	args := []any{since}
	if typeFilter != "" {
		q += ` AND type = $2`
		args = append(args, typeFilter)
	}
	q += fmt.Sprintf(` ORDER BY created_at ASC LIMIT $%d`, len(args)+1)
	args = append(args, limit)

	rows, err := b.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("replay query: %w", err)
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		var ev Event
		if err := rows.Scan(&ev.ID, &ev.Type, &ev.OrganizationID, &ev.ProjectID,
			&ev.Payload, &ev.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, ev)
	}
	return out, rows.Err()
}

// ErrUnsubscribe se usa para desuscribir handlers (no implementado en MVP).
var ErrUnsubscribe = errors.New("unsubscribe not implemented in MVP")
