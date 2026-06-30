// Package observability agrega hooks runtime sobre el MCP server:
// tool invocations, sql slow queries, fn calls, resource snapshots.
//
// issue-53.1 mvp: tool invocations — slog + INSERT async a mcp_tool_invocations.
package observability

import (
	"context"
	"crypto/sha256"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Invocation describe UNA tool call. Los campos UUID Nil quedan NULL en BD.
type Invocation struct {
	ToolName     string
	PrincipalID  uuid.UUID
	OrgID        uuid.UUID
	ProjectID    uuid.UUID
	Status       string
	DurationMS   int
	ErrorCode    string
	ErrorMessage string
	ArgsHash     []byte
	WorkflowID   string
}

// InvocationStore es la abstraccion minima para persistir invocaciones.
// Definida en el consumidor (el worker la invoca) para poder mockear en tests.
type InvocationStore interface {
	InsertInvocation(ctx context.Context, inv Invocation) error
}

// PGInvocationStore persiste invocaciones en mcp_tool_invocations.
type PGInvocationStore struct {
	Pool *pgxpool.Pool
}

// InsertInvocation ejecuta el INSERT en mcp_tool_invocations.
func (s *PGInvocationStore) InsertInvocation(ctx context.Context, inv Invocation) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO mcp_tool_invocations (
			tool_name, principal_id, org_id, project_id, status, duration_ms,
			error_code, error_message, args_hash, workflow_id
		) VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7,''), NULLIF($8,''), $9, NULLIF($10,''))
	`,
		inv.ToolName,
		nullableUUID(inv.PrincipalID),
		nullableUUID(inv.OrgID),
		nullableUUID(inv.ProjectID),
		inv.Status,
		inv.DurationMS,
		inv.ErrorCode,
		inv.ErrorMessage,
		inv.ArgsHash,
		inv.WorkflowID,
	)
	return err
}

// InvocationLogger encola invocaciones y las persiste async via worker pool.
// Si la cola esta llena, dropea con WARN (no bloquea el handler).
type InvocationLogger struct {
	store    InvocationStore
	logger   *slog.Logger
	queue    chan Invocation
	workers  int
	closeMu  sync.Mutex
	done     chan struct{}
	wg       sync.WaitGroup
	insertTO time.Duration
}

const (
	defaultWorkers  = 4
	defaultQueueCap = 1024
	defaultTimeout  = 2 * time.Second
)

// NewInvocationLogger arranca N workers que consumen del canal.
// workers<=0 -> defaultWorkers. queueSize<=0 -> defaultQueueCap.
// store==nil -> panic (caller error). logger==nil -> slog.Default().
func NewInvocationLogger(store InvocationStore, logger *slog.Logger, workers, queueSize int) *InvocationLogger {
	if workers <= 0 {
		workers = defaultWorkers
	}
	if queueSize <= 0 {
		queueSize = defaultQueueCap
	}
	if logger == nil {
		logger = slog.Default()
	}
	l := &InvocationLogger{
		store:    store,
		logger:   logger,
		queue:    make(chan Invocation, queueSize),
		workers:  workers,
		done:     make(chan struct{}),
		insertTO: defaultTimeout,
	}
	for i := 0; i < workers; i++ {
		l.wg.Add(1)
		go l.worker()
	}
	return l
}

// HashArgs computa sha256 de los argumentos para no persistir PII.
func HashArgs(args []byte) []byte {
	if len(args) == 0 {
		return nil
	}
	sum := sha256.Sum256(args)
	return sum[:]
}

// Log enqueuea una invocacion. No-bloqueante: si el canal esta lleno, dropea + WARN.
// Si el logger ya cerro (post-shutdown), drop silencioso.
func (l *InvocationLogger) Log(inv Invocation) {
	select {
	case <-l.done:
		return
	default:
	}
	select {
	case l.queue <- inv:
	default:
		l.logger.Warn("invocation queue full, dropping",
			slog.String("tool", inv.ToolName),
			slog.String("status", inv.Status))
	}
}

// Close senala el cierre y espera el drain final. Idempotente.
// NO cierra el canal `queue` (evita race con producers en vuelo); los
// workers drenan lo que queda al ver `done` y terminan.
func (l *InvocationLogger) Close() {
	l.closeMu.Lock()
	select {
	case <-l.done:
		l.closeMu.Unlock()
		return
	default:
		close(l.done)
	}
	l.closeMu.Unlock()
	l.wg.Wait()
}

func (l *InvocationLogger) worker() {
	defer l.wg.Done()
	for {
		select {
		case inv := <-l.queue:
			l.persist(inv)
		case <-l.done:
			l.drain()
			return
		}
	}
}

func (l *InvocationLogger) drain() {
	for {
		select {
		case inv := <-l.queue:
			l.persist(inv)
		default:
			return
		}
	}
}

func (l *InvocationLogger) persist(inv Invocation) {
	ctx, cancel := context.WithTimeout(context.Background(), l.insertTO)
	defer cancel()
	if err := l.store.InsertInvocation(ctx, inv); err != nil {
		l.logger.Warn("invocation persist failed",
			slog.String("tool", inv.ToolName),
			slog.String("status", inv.Status),
			slog.String("error", err.Error()))
	}
}

func nullableUUID(u uuid.UUID) any {
	if u == uuid.Nil {
		return nil
	}
	return u
}
