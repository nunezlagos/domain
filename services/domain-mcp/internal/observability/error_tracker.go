// Package observability: este archivo cubre el registro y dedup de errores
// para early-error-reporting. Record categoriza, calcula fingerprint y encola
// el evento; un worker async hace el upsert con dedup y, tras un upsert
// exitoso, dispara los hooks de alerting y self-heal.
//
// issue-53.9 early-error-reporting.
package observability

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrorEvent es el payload a persistir/deduplicar en error_events.
type ErrorEvent struct {
	Source      string
	Category    Category
	Severity    string
	Message     string
	StackTrace  string
	Fingerprint []byte
	WorkflowID  string
	ProjectID   string
}

// ErrorEventStore abstrae la persistencia con dedup.
type ErrorEventStore interface {
	UpsertErrorEvent(ctx context.Context, e ErrorEvent) error
}

// AlertHook se invoca tras un upsert exitoso (alerting / self-heal desacoplados).
type AlertHook func(ctx context.Context, e ErrorEvent)

// ErrorTracker registra errores categorizados con dedup por fingerprint.
// Record es NO-bloqueante: encola y un pool de workers persiste async (mismo
// patron que SlowQueryTracer), por lo que es seguro en hot-paths.
type ErrorTracker struct {
	store     ErrorEventStore
	logger    *slog.Logger
	onAlert   AlertHook
	onHeal    AlertHook
	queue     chan ErrorEvent
	done      chan struct{}
	wg        sync.WaitGroup
	closeMu   sync.Mutex
	persistTO time.Duration
}

// NewErrorTracker construye el tracker y arranca sus workers.
// logger nil -> slog.Default(). workers<=0 -> defaultWorkers.
func NewErrorTracker(store ErrorEventStore, logger *slog.Logger) *ErrorTracker {
	if logger == nil {
		logger = slog.Default()
	}
	t := &ErrorTracker{
		store:     store,
		logger:    logger,
		queue:     make(chan ErrorEvent, defaultQueueCap),
		done:      make(chan struct{}),
		persistTO: defaultTimeout,
	}
	for i := 0; i < defaultWorkers; i++ {
		t.wg.Add(1)
		go t.worker()
	}
	return t
}

// SetAlertHook registra el callback de alerting (idempotente).
func (t *ErrorTracker) SetAlertHook(h AlertHook) { t.onAlert = h }

// SetHealHook registra el callback de self-heal (idempotente).
func (t *ErrorTracker) SetHealHook(h AlertHook) { t.onHeal = h }

// Record categoriza el error, calcula su fingerprint y lo encola (no bloquea).
// Si la cola esta llena, dropea con WARN. Seguro para hot-paths.
func (t *ErrorTracker) Record(ctx context.Context, err error, source string) {
	if err == nil {
		return
	}
	cat := Categorize(err)
	e := ErrorEvent{
		Source:      source,
		Category:    cat,
		Severity:    defaultSeverity(cat),
		Message:     err.Error(),
		Fingerprint: Fingerprint(cat, err.Error(), source, ""),
		WorkflowID:  WorkflowIDFromContext(ctx).String(),
	}
	select {
	case <-t.done:
		return
	default:
	}
	select {
	case t.queue <- e:
	default:
		t.logger.Warn("error event queue full, dropping",
			slog.String("source", source), slog.String("category", string(cat)))
	}
}

func (t *ErrorTracker) worker() {
	defer t.wg.Done()
	for {
		select {
		case e := <-t.queue:
			t.persist(e, true)
		case <-t.done:
			t.drain()
			return
		}
	}
}

// drain procesa lo encolado al cerrar. Persiste los eventos pendientes (dato
// valioso) pero NO dispara los hooks: en shutdown alerting/self-heal son
// inutiles y, al ser fire-and-forget, dejarian goroutines corriendo contra un
// pool que ya se va a cerrar.
func (t *ErrorTracker) drain() {
	for {
		select {
		case e := <-t.queue:
			t.persist(e, false)
		default:
			return
		}
	}
}

// persist hace el upsert y, si sale OK y fireHooks, dispara alerting y self-heal.
func (t *ErrorTracker) persist(e ErrorEvent, fireHooks bool) {
	ctx, cancel := context.WithTimeout(context.Background(), t.persistTO)
	defer cancel()
	if err := t.store.UpsertErrorEvent(ctx, e); err != nil {
		t.logger.Warn("error event persist failed",
			slog.String("source", e.Source),
			slog.String("category", string(e.Category)),
			slog.String("error", err.Error()))
		return
	}
	if !fireHooks {
		return
	}
	if t.onAlert != nil {
		t.onAlert(ctx, e)
	}
	if t.onHeal != nil {
		t.onHeal(ctx, e)
	}
}

// Close senala el cierre y espera el drain final. Idempotente.
func (t *ErrorTracker) Close() {
	t.closeMu.Lock()
	select {
	case <-t.done:
		t.closeMu.Unlock()
		return
	default:
		close(t.done)
	}
	t.closeMu.Unlock()
	t.wg.Wait()
}

// defaultSeverity mapea la categoria a una severidad inicial razonable.
// El operador puede sobreescribir por known_error.
func defaultSeverity(cat Category) string {
	switch cat {
	case CategoryPanic:
		return "critical"
	case CategorySQL, CategoryAuth, CategoryExternal:
		return "error"
	default:
		return "warn"
	}
}

// PGErrorEventStore persiste con dedup en error_events.
// Pool puede ser nil al inicio; setear via SetPool post-OpenProduction.
type PGErrorEventStore struct {
	Pool *pgxpool.Pool
}

// SetPool setea el pool (post-init).
func (s *PGErrorEventStore) SetPool(p *pgxpool.Pool) { s.Pool = p }

// UpsertErrorEvent inserta el evento o, si el fingerprint ya existe,
// incrementa dedup_count y refresca last_seen_at.
func (s *PGErrorEventStore) UpsertErrorEvent(ctx context.Context, e ErrorEvent) error {
	if s.Pool == nil {
		return ErrStoreNotReady
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO error_events
			(source, category, severity, message, stack_trace, fingerprint, workflow_id, project_id)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,NULLIF($7,''),NULLIF($8,'')::uuid)
		ON CONFLICT (fingerprint) DO UPDATE
		SET dedup_count = error_events.dedup_count + 1,
		    last_seen_at = now()
	`,
		e.Source, string(e.Category), e.Severity, e.Message,
		e.StackTrace, e.Fingerprint, e.WorkflowID, e.ProjectID,
	)
	return err
}
