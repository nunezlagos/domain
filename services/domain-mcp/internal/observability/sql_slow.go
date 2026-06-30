// Package observability: este archivo cubre el dominio SQL slow queries.
// Captura cualquier query con duration > threshold y la persiste async.
//
// issue-53.6 comprehensive.
package observability

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
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
type PGSlowQueryStore struct {
	Pool *pgxpool.Pool
}

// InsertSlowQuery ejecuta el INSERT.
func (s *PGSlowQueryStore) InsertSlowQuery(ctx context.Context, q SlowQuery) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO sql_slow_queries (query_text, args_hash, duration_ms, plan_text, workflow_id)
		VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''))
	`,
		q.QueryText, q.ArgsHash, q.DurationMS, q.PlanText, q.WorkflowID,
	)
	return err
}

// SlowQueryTracer implementa pgx.QueryTracer; captura duration por query y
// enqueuea las que exceden threshold. Delega al inner tracer para mantener
// compatibilidad con SQLErrorCaptureTracer del proyecto (HU 51.1).
type SlowQueryTracer struct {
	inner     pgx.QueryTracer
	store     SlowQueryStore
	logger    *slog.Logger
	queue     chan SlowQuery
	workers   int
	closeMu   sync.Mutex
	closed    bool
	wg        sync.WaitGroup
	threshold time.Duration
	persistTO time.Duration
}

type slowStartKey struct{}

// SlowThresholdDefaultMs es el threshold por default para considerar una query "slow".
const SlowThresholdDefaultMs = 100

// NewSlowQueryTracer arranca workers que procesan slow queries.
// thresholdMs<=0 -> SlowThresholdDefaultMs. workers<=0 -> defaultWorkers.
func NewSlowQueryTracer(inner pgx.QueryTracer, store SlowQueryStore, logger *slog.Logger, workers int, thresholdMs int) *SlowQueryTracer {
	if workers <= 0 {
		workers = defaultWorkers
	}
	if thresholdMs <= 0 {
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

// Close flush + wait. Idempotente.
func (t *SlowQueryTracer) Close() {
	t.closeMu.Lock()
	if t.closed {
		t.closeMu.Unlock()
		return
	}
	t.closed = true
	close(t.queue)
	t.closeMu.Unlock()
	t.wg.Wait()
}

func (t *SlowQueryTracer) worker() {
	defer t.wg.Done()
	for q := range t.queue {
		t.persist(q)
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

// TraceQueryStart delega al inner y setea start time en ctx.
func (t *SlowQueryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	ctx = t.inner.TraceQueryStart(ctx, conn, data)
	return context.WithValue(ctx, slowStartKey{}, time.Now())
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
	select {
	case t.queue <- SlowQuery{
		QueryText:  data.SQL,
		DurationMS: dur.Milliseconds(),
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
