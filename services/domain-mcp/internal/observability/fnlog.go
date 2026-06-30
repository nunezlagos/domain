// Package observability: este archivo cubre el dominio fnlog decorator
// (instrumentacion de llamadas internas entre paquetes).
//
// issue-53.6 comprehensive.
package observability

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// FnLogEntry es el payload de UNA llamada a una fn interna.
type FnLogEntry struct {
	FnName       string
	Pkg          string
	ArgsHash     []byte
	DurationUS   int64
	Status       string
	ErrorMessage string
	WorkflowID   string
}

// FnLogStore abstrae la persistencia de fn logs.
type FnLogStore interface {
	InsertFnLog(ctx context.Context, e FnLogEntry) error
}

// PGFnLogStore persiste en function_calls.
type PGFnLogStore struct {
	Pool *pgxpool.Pool
}

// InsertFnLog ejecuta el INSERT.
func (s *PGFnLogStore) InsertFnLog(ctx context.Context, e FnLogEntry) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO function_calls (fn_name, pkg, args_hash, duration_us, status, error_message, workflow_id)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6,''), NULLIF($7,''))
	`,
		e.FnName, e.Pkg, e.ArgsHash, e.DurationUS, e.Status, e.ErrorMessage, e.WorkflowID,
	)
	return err
}

// FnLogger es un handle para instrumentar fn calls internas.
// Uso:
//
//	defer fl.Enter("observation.Save", args)(err)
//	o
//	defer fl.Enter("observation.Save", args)(nil)
type FnLogger struct {
	store   FnLogStore
	logger  *slog.Logger
	queue   chan FnLogEntry
	workers int
	closeMu sync.Mutex
	done    chan struct{}
	wg      sync.WaitGroup
}

// NewFnLogger arranca N workers.
// workers<=0 -> defaultWorkers.
func NewFnLogger(store FnLogStore, logger *slog.Logger, workers int) *FnLogger {
	if workers <= 0 {
		workers = defaultWorkers
	}
	if logger == nil {
		logger = slog.Default()
	}
	f := &FnLogger{
		store:   store,
		logger:  logger,
		queue:   make(chan FnLogEntry, defaultQueueCap),
		workers: workers,
		done:    make(chan struct{}),
	}
	for i := 0; i < workers; i++ {
		f.wg.Add(1)
		go f.worker()
	}
	return f
}

// Close senala el cierre y espera el drain final. Idempotente.
// NO cierra el canal `queue` (evita race con producers en vuelo).
func (f *FnLogger) Close() {
	f.closeMu.Lock()
	select {
	case <-f.done:
		f.closeMu.Unlock()
		return
	default:
		close(f.done)
	}
	f.closeMu.Unlock()
	f.wg.Wait()
}

func (f *FnLogger) worker() {
	defer f.wg.Done()
	for {
		select {
		case e := <-f.queue:
			f.persist(e)
		case <-f.done:
			f.drain()
			return
		}
	}
}

func (f *FnLogger) drain() {
	for {
		select {
		case e := <-f.queue:
			f.persist(e)
		default:
			return
		}
	}
}

func (f *FnLogger) persist(e FnLogEntry) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	if err := f.store.InsertFnLog(ctx, e); err != nil {
		f.logger.Warn("fn log persist failed",
			slog.String("fn", e.FnName),
			slog.String("status", e.Status),
			slog.String("error", err.Error()))
	}
}

// Enter registra el inicio y retorna un callback Exit que el caller usa
// con defer. La exit captura duracion y error final.
func (f *FnLogger) Enter(fnName, pkg string, args []byte) func(error) {
	start := time.Now()
	return func(err error) {
		status := "ok"
		msg := ""
		if err != nil {
			status = "error"
			msg = err.Error()
		}
		select {
		case <-f.done:
			return
		default:
		}
		select {
		case f.queue <- FnLogEntry{
			FnName:       fnName,
			Pkg:          pkg,
			ArgsHash:     HashArgs(args),
			DurationUS:   time.Since(start).Microseconds(),
			Status:       status,
			ErrorMessage: msg,
			WorkflowID:   "",
		}:
		default:
			f.logger.Warn("fn log queue full, dropping",
				slog.String("fn", fnName),
				slog.String("status", status))
		}
	}
}

// Trace se usa para capturar panics automaticos. Convierte panic en error y
// registra con status=panic.
func (f *FnLogger) Trace(fnName, pkg string, args []byte, fn func() error) error {
	exit := f.Enter(fnName, pkg, args)
	err := safeCall(fn)
	exit(err)
	return err
}

// safeCall ejecuta fn y captura panic, devolviendo un error en su lugar.
func safeCall(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
				return
			}
			err = panicError{value: r}
		}
	}()
	return fn()
}

type panicError struct{ value any }

func (p panicError) Error() string {
	return "panic: " + toString(p.value)
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return "non-string panic"
}
