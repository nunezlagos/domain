// Package observability agrega hooks runtime sobre el MCP server:
// tool invocations, http requests, internal fn calls, sql slow queries,
// resource snapshots.
//
// issue-53.6 comprehensive: este archivo cubre el dominio HTTP request log.
package observability

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HTTPLog es el payload de UNA HTTP request.
type HTTPLog struct {
	RequestID   uuid.UUID
	Method      string
	Path        string
	Status      int
	DurationMS  int64
	PrincipalID uuid.UUID
	BytesIn     int
	BytesOut    int
	UserAgent   string
	WorkflowID  string
}

// HTTPLogStore abstrae la persistencia de HTTP logs.
type HTTPLogStore interface {
	InsertHTTPLog(ctx context.Context, log HTTPLog) error
}

// PGHTTPLogStore persiste en http_request_log.
type PGHTTPLogStore struct {
	Pool *pgxpool.Pool
}

// InsertHTTPLog ejecuta el INSERT.
func (s *PGHTTPLogStore) InsertHTTPLog(ctx context.Context, l HTTPLog) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO http_request_log (
			request_id, method, path, status, duration_ms,
			principal_id, bytes_in, bytes_out, user_agent, workflow_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9,''), NULLIF($10,''))
	`,
		l.RequestID, l.Method, l.Path, l.Status, l.DurationMS,
		nullableUUID(l.PrincipalID), l.BytesIn, l.BytesOut, l.UserAgent, l.WorkflowID,
	)
	return err
}

// HTTPLogger es un middleware HTTP que enqueuea cada request a un worker pool.
// Non-blocking: si la cola se llena, dropea + WARN.
type HTTPLogger struct {
	store   HTTPLogStore
	logger  *slog.Logger
	queue   chan HTTPLog
	workers int
	closeMu sync.Mutex
	done    chan struct{}
	wg      sync.WaitGroup
}

// NewHTTPLogger arranca N workers y retorna el middleware listo.
// workers<=0 -> 4.
func NewHTTPLogger(store HTTPLogStore, logger *slog.Logger, workers int) *HTTPLogger {
	if workers <= 0 {
		workers = defaultWorkers
	}
	if logger == nil {
		logger = slog.Default()
	}
	h := &HTTPLogger{
		store:   store,
		logger:  logger,
		queue:   make(chan HTTPLog, defaultQueueCap),
		workers: workers,
		done:    make(chan struct{}),
	}
	for i := 0; i < workers; i++ {
		h.wg.Add(1)
		go h.worker()
	}
	return h
}

// Close senala el cierre y espera el drain final. Idempotente.
// NO cierra el canal `queue` (evita race con producers en vuelo).
func (h *HTTPLogger) Close() {
	h.closeMu.Lock()
	select {
	case <-h.done:
		h.closeMu.Unlock()
		return
	default:
		close(h.done)
	}
	h.closeMu.Unlock()
	h.wg.Wait()
}

func (h *HTTPLogger) worker() {
	defer h.wg.Done()
	for {
		select {
		case entry := <-h.queue:
			h.persist(entry)
		case <-h.done:
			h.drain()
			return
		}
	}
}

func (h *HTTPLogger) drain() {
	for {
		select {
		case entry := <-h.queue:
			h.persist(entry)
		default:
			return
		}
	}
}

func (h *HTTPLogger) persist(l HTTPLog) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	if err := h.store.InsertHTTPLog(ctx, l); err != nil {
		h.logger.Warn("http log persist failed",
			slog.String("path", l.Path),
			slog.Int("status", l.Status),
			slog.String("error", err.Error()))
	}
}

// enqueue non-blocking; drop + WARN si saturado.
// Si el logger ya cerro, drop silencioso.
func (h *HTTPLogger) enqueue(l HTTPLog) {
	select {
	case <-h.done:
		return
	default:
	}
	select {
	case h.queue <- l:
	default:
		h.logger.Warn("http log queue full, dropping",
			slog.String("path", l.Path),
			slog.Int("status", l.Status))
	}
}

// Middleware retorna un wrapper HTTP que captura cada request y lo enqueuea.
// Tambien agrega X-Request-Id y X-Workflow-Id a la response. Lee X-Workflow-Id
// del request si viene, sino genera uno y lo propaga en ctx.
func (h *HTTPLogger) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := uuid.New()
		wfID := WorkflowIDFromContext(r.Context())
		if wfID == uuid.Nil {
			if hdr := r.Header.Get("X-Workflow-Id"); hdr != "" {
				if parsed, err := uuid.Parse(hdr); err == nil {
					wfID = parsed
				}
			}
			if wfID == uuid.Nil {
				wfID = NewWorkflowID()
			}
		}
		rw := &statusWriter{ResponseWriter: w, status: http.StatusOK}

		ctx := context.WithValue(r.Context(), requestIDKey{}, requestID)
		ctx = WithWorkflowID(ctx, wfID)
		next.ServeHTTP(rw, r.WithContext(ctx))

		dur := time.Since(start).Milliseconds()
		rw.Header().Set("X-Request-Id", requestID.String())
		rw.Header().Set("X-Workflow-Id", wfID.String())

		h.enqueue(HTTPLog{
			RequestID:  requestID,
			Method:     r.Method,
			Path:       r.URL.Path,
			Status:     rw.status,
			DurationMS: dur,
			BytesIn:    int(r.ContentLength),
			BytesOut:   rw.bytes,
			UserAgent:  r.UserAgent(),
			WorkflowID: wfID.String(),
		})
	})
}

// requestIDKey es la clave privada en ctx para propagar request_id.
type requestIDKey struct{}

// RequestIDFromContext devuelve el UUID seteado por el middleware. uuid.Nil si ausente.
func RequestIDFromContext(ctx context.Context) uuid.UUID {
	if v, ok := ctx.Value(requestIDKey{}).(uuid.UUID); ok {
		return v
	}
	return uuid.Nil
}

// statusWriter captura el status code y los bytes escritos.
type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusWriter) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}
