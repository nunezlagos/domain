// Package observability: este archivo cubre el dominio SQL slow queries.
// Captura cualquier query con duration > threshold y la persiste async.
//
// issue-53.6 comprehensive.
package observability

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SlowQuery es el payload a persistir en sql_slow_queries.
type SlowQuery struct {
	QueryText  string
	ArgsHash   []byte
	DurationMS int64
	PlanText   string
	WorkflowID string
}

// SlowQueryStore abstrae la persistencia.
type SlowQueryStore interface {
	InsertSlowQuery(ctx context.Context, q SlowQuery) error
}

// PGSlowQueryStore persiste en sql_slow_queries.
// Pool puede ser nil al inicio; setear via SetPool post-OpenProduction
// cuando el pgxpool.Pool esta disponible.
type PGSlowQueryStore struct {
	Pool *pgxpool.Pool
}

// SetPool setea el pool (post-init).
func (s *PGSlowQueryStore) SetPool(p *pgxpool.Pool) {
	s.Pool = p
}

// InsertSlowQuery ejecuta el INSERT. Si Pool es nil, dropea + WARN.
func (s *PGSlowQueryStore) InsertSlowQuery(ctx context.Context, q SlowQuery) error {
	if s.Pool == nil {
		return ErrStoreNotReady
	}
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO sql_slow_queries (query_text, args_hash, duration_ms, plan_text, workflow_id)
		VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''))
	`,
		q.QueryText, q.ArgsHash, q.DurationMS, q.PlanText, q.WorkflowID,
	)
	return err
}

// ErrStoreNotReady indica que el store aun no recibio su pool.
var ErrStoreNotReady = errors.New("observability: slow query store pool not set; call SetPool after db.Open*")

// WireSlowQueryTracer construye un SlowQueryTracer listo para encadenar
// a un pgxpool.Pool. Devuelve (tracer, store). El caller debe:
//
//	tracer, store := observability.WireSlowQueryTracer(logger)
//	db.SetObservabilityTracer(tracer)         // ANTES de db.Open*
//	pools, ... := db.OpenProduction(...)
//	store.SetPool(pools.App)                    // DESPUES de Open*
//
// thresholdMs<0 usa el default (100ms); DOMAIN_SQL_SLOW_THRESHOLD_MS via env.
type SlowQueryTracer struct {
	inner     pgx.QueryTracer
	store     SlowQueryStore
	logger    *slog.Logger
	queue     chan SlowQuery
	workers   int
	closeMu   sync.Mutex
	done      chan struct{}
	wg        sync.WaitGroup
	threshold time.Duration
	persistTO time.Duration
}

type slowStartKey struct{}
type slowSQLKey struct{}

// SlowThresholdDefaultMs es el threshold por default para considerar una query "slow".
const SlowThresholdDefaultMs = 100

// NewSlowQueryTracer arranca workers que procesan slow queries.
// thresholdMs<0 -> SlowThresholdDefaultMs (100). thresholdMs==0 -> sin threshold (toda query cuenta).
// workers<=0 -> defaultWorkers.
func NewSlowQueryTracer(inner pgx.QueryTracer, store SlowQueryStore, logger *slog.Logger, workers int, thresholdMs int) *SlowQueryTracer {
	if workers <= 0 {
		workers = defaultWorkers
	}
	if thresholdMs < 0 {
		thresholdMs = SlowThresholdDefaultMs
	}
	if logger == nil {
		logger = slog.Default()
	}
	if inner == nil {
		inner = noopTracer{}
	}
	t := &SlowQueryTracer{
		inner:     inner,
		store:     store,
		logger:    logger,
		queue:     make(chan SlowQuery, defaultQueueCap),
		workers:   workers,
		done:      make(chan struct{}),
		threshold: time.Duration(thresholdMs) * time.Millisecond,
		persistTO: defaultTimeout,
	}
	for i := 0; i < workers; i++ {
		t.wg.Add(1)
		go t.worker()
	}
	return t
}

// NewFromEnv permite configurar el threshold via DOMAIN_SQL_SLOW_THRESHOLD_MS.
func NewFromEnv(inner pgx.QueryTracer, store SlowQueryStore, logger *slog.Logger, workers int) *SlowQueryTracer {
	threshold := SlowThresholdDefaultMs
	if v := os.Getenv("DOMAIN_SQL_SLOW_THRESHOLD_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			threshold = n
		}
	}
	return NewSlowQueryTracer(inner, store, logger, workers, threshold)
}

// Close senala el cierre y espera el drain final. Idempotente.
// NO cierra el canal `queue` (evita race con producers en vuelo).
func (t *SlowQueryTracer) Close() {
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

func (t *SlowQueryTracer) worker() {
	defer t.wg.Done()
	for {
		select {
		case q := <-t.queue:
			t.persist(q)
		case <-t.done:
			t.drain()
			return
		}
	}
}

func (t *SlowQueryTracer) drain() {
	for {
		select {
		case q := <-t.queue:
			t.persist(q)
		default:
			return
		}
	}
}

func (t *SlowQueryTracer) persist(q SlowQuery) {
	ctx, cancel := context.WithTimeout(context.Background(), t.persistTO)
	defer cancel()
	if err := t.store.InsertSlowQuery(ctx, q); err != nil {
		t.logger.Warn("slow query persist failed",
			slog.Int64("duration_ms", q.DurationMS),
			slog.String("error", err.Error()))
	}
}

// TraceQueryStart delega al inner y setea start time + SQL en ctx.
// pgx v5 TraceQueryEndData NO incluye SQL — guardamos en ctx para recuperarlo en TraceQueryEnd.
func (t *SlowQueryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	ctx = t.inner.TraceQueryStart(ctx, conn, data)
	ctx = context.WithValue(ctx, slowStartKey{}, time.Now())
	ctx = context.WithValue(ctx, slowSQLKey{}, data.SQL)
	return ctx
}

// TraceQueryEnd delega al inner, mide duracion y enqueuea si lento.
func (t *SlowQueryTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	t.inner.TraceQueryEnd(ctx, conn, data)
	start, ok := ctx.Value(slowStartKey{}).(time.Time)
	if !ok {
		return
	}
	dur := time.Since(start)
	if dur < t.threshold {
		return
	}
	sqlText, _ := ctx.Value(slowSQLKey{}).(string)
	wfID := WorkflowIDFromContext(ctx)
	select {
	case <-t.done:
		return
	default:
	}
	select {
	case t.queue <- SlowQuery{
		QueryText:  sqlText,
		DurationMS: dur.Milliseconds(),
		WorkflowID: wfID.String(),
	}:
	default:
		t.logger.Warn("slow query queue full, dropping",
			slog.Int64("duration_ms", dur.Milliseconds()))
	}
}

// noopTracer es el fallback si el caller no pasa inner.
type noopTracer struct{}

func (noopTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	return ctx
}
func (noopTracer) TraceQueryEnd(_ context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {}

// WireSlowQueryTracer construye un SlowQueryTracer listo para encadenar
// a un pgxpool.Pool. Devuelve (tracer, store). El caller debe:
//
//	tracer, store := observability.WireSlowQueryTracer(inner, logger, workers)
//	db.SetObservabilityTracer(tracer)         // ANTES de db.Open*
//	pools, _ := db.OpenProduction(...)
//	store.SetPool(pools.App)                    // DESPUES de Open*
//
// thresholdMs<0 usa el default (100ms); DOMAIN_SQL_SLOW_THRESHOLD_MS via env.
// `inner` debe ser el SQLErrorCaptureTracer para preservar HU 51.1.
func WireSlowQueryTracer(inner pgx.QueryTracer, logger *slog.Logger, workers int) (*SlowQueryTracer, *PGSlowQueryStore) {
	store := &PGSlowQueryStore{}
	t := NewFromEnv(inner, store, logger, workers)
	return t, store
}
