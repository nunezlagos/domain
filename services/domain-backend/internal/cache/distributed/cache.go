// Package distributed — issue-26.7 patrón uniforme de invalidación de cache
// in-memory cross-pod via Postgres LISTEN/NOTIFY.
//
// Casos de uso: platform_policies (issue-01.8),
// mcp_servers (issue-12.6), plans+custom_limits (issue-21.3), model_registry
// pricing, agent definitions LRU.
//
// Convención naming de channels: cache_invalidate_<entity>.
// Payload: {operation:"insert|update|delete", id:"<uuid>", organization_id:"<uuid>"}.
package distributed

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Cache es la interfaz mínima que cualquier cache local debe satisfacer
// para poder envolverla con WithInvalidation.
type Cache interface {
	// InvalidateKey borra una entrada específica.
	InvalidateKey(key string)
	// InvalidateAll vacía toda la cache (fallback si payload no parseable).
	InvalidateAll()
}

// Payload es el shape JSON que viaja por el channel.
type Payload struct {
	Operation      string     `json:"operation"`                  // insert | update | delete
	ID             string     `json:"id"`                         // entity primary key
	OrganizationID *string    `json:"organization_id,omitempty"`  // scope opcional
	Extra          map[string]any `json:"extra,omitempty"`        // metadata libre
}

// ChannelFor devuelve el nombre canónico de channel para una entidad.
// Convención: cache_invalidate_<entity_plural_snake_case>.
func ChannelFor(entity string) string {
	return "cache_invalidate_" + entity
}

// Listener escucha un channel y ejecuta handlers registrados.
// Reconexión automática con backoff exponencial sobre fallos transitorios.
type Listener struct {
	Pool    *pgxpool.Pool
	Channel string
	Logger  *slog.Logger
	Cache   Cache // cache a invalidar cuando llega un payload

	mu        sync.Mutex
	handlers  []func(context.Context, Payload)
	stopCh    chan struct{}
	stopped   bool
}

// NewListener construye un Listener sobre un Pool + channel + cache local.
func NewListener(pool *pgxpool.Pool, channel string, cache Cache, logger *slog.Logger) *Listener {
	if logger == nil {
		logger = slog.Default()
	}
	return &Listener{
		Pool:    pool,
		Channel: channel,
		Cache:   cache,
		Logger:  logger,
		stopCh:  make(chan struct{}),
	}
}

// OnPayload registra un handler adicional para cada payload (post-invalidación).
// Útil para métricas o triggers de re-fetch.
func (l *Listener) OnPayload(fn func(context.Context, Payload)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.handlers = append(l.handlers, fn)
}

// Start arranca el listener en background. Llamada idempotente.
// Bloquea hasta que el contexto sea cancelado o Stop() sea llamado.
func (l *Listener) Start(ctx context.Context) error {
	backoff := time.Second
	const maxBackoff = 30 * time.Second

	for {
		select {
		case <-l.stopCh:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := l.listenLoop(ctx)
		if err == nil || err == context.Canceled {
			return err
		}
		l.Logger.Warn("cache listener disconnected, reconnecting",
			slog.String("channel", l.Channel),
			slog.Any("err", err),
			slog.Duration("backoff", backoff),
		)

		// ISSUE-28.8: NewTimer reusable en retry loop (time.After leak).
		bt := time.NewTimer(backoff)
		select {
		case <-l.stopCh:
			bt.Stop()
			return nil
		case <-ctx.Done():
			bt.Stop()
			return ctx.Err()
		case <-bt.C:
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// Stop señala al listener que termine. Idempotente.
func (l *Listener) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stopped {
		return
	}
	l.stopped = true
	close(l.stopCh)
}

func (l *Listener) listenLoop(ctx context.Context) error {
	conn, err := l.Pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN "+pgx.Identifier{l.Channel}.Sanitize()); err != nil {
		return fmt.Errorf("LISTEN %s: %w", l.Channel, err)
	}
	l.Logger.Info("cache listener active", slog.String("channel", l.Channel))

	for {
		select {
		case <-l.stopCh:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		notif, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			return fmt.Errorf("wait notification: %w", err)
		}
		if notif == nil {
			continue
		}

		var payload Payload
		if err := json.Unmarshal([]byte(notif.Payload), &payload); err != nil {
			l.Logger.Warn("invalid cache invalidation payload; invalidating all (fail-safe)",
				slog.String("channel", l.Channel),
				slog.String("raw", notif.Payload),
				slog.Any("err", err),
			)
			l.Cache.InvalidateAll()
			continue
		}

		// Invalidación granular por ID. operation guía si fue insert/update/delete
		// pero todos invalidan la entrada del cache (el cliente re-fetchea si necesita).
		if payload.ID != "" {
			l.Cache.InvalidateKey(payload.ID)
		} else {
			l.Cache.InvalidateAll()
		}

		l.mu.Lock()
		handlers := append([]func(context.Context, Payload){}, l.handlers...)
		l.mu.Unlock()
		for _, h := range handlers {
			h(ctx, payload)
		}
	}
}

// Publish envía un NOTIFY al channel. Útil para invocar invalidación
// programáticamente desde el service layer (alternativa a triggers DB).
func Publish(ctx context.Context, pool *pgxpool.Pool, channel string, payload Payload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	_, err = pool.Exec(ctx, "SELECT pg_notify($1, $2)", channel, string(body))
	if err != nil {
		return fmt.Errorf("pg_notify: %w", err)
	}
	return nil
}

// PublishEntityChange es helper para emitir un payload estándar.
func PublishEntityChange(ctx context.Context, pool *pgxpool.Pool, entity, operation string, id uuid.UUID, orgID *uuid.UUID) error {
	var orgStr *string
	if orgID != nil {
		s := orgID.String()
		orgStr = &s
	}
	return Publish(ctx, pool, ChannelFor(entity), Payload{
		Operation:      operation,
		ID:             id.String(),
		OrganizationID: orgStr,
	})
}
