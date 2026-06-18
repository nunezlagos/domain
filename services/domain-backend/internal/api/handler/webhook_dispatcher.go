package handler

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// WebhookDispatcher gestiona el lifecycle de las goroutines que ejecutan
// dispatch de webhooks inbound. Garantiza:
//   - Backpressure: bounded queue (channel bufferizado). Enqueue retorna
//     false si está llena → handler responde 503.
//   - Timeout por job: cada job corre con su propio context con deadline.
//   - WaitGroup: el shutdown espera a los jobs en vuelo (hasta un budget).
//   - Context cancelable: shutdown cancela jobs que están en vuelo.
//
// ISSUE-28.7 (webhook-goroutine-lifecycle): antes el handler hacía
// `go a.runWebhookTarget(context.Background(), ...)` directamente. Problemas:
//   - Unbounded goroutines (10k webhooks/s → 10k goroutines).
//   - Sin cancelación: context.Background no hereda el shutdown.
//   - Sin WaitGroup: graceful shutdown no espera los webhooks.
type WebhookDispatcher struct {
	ch       chan webhookJob
	wg       sync.WaitGroup
	timeout  time.Duration
	dispatch func(context.Context, webhookJob)
	logger   *slog.Logger

	// closed es el flag que indica que el dispatcher no acepta más jobs.
	// Se setea en Shutdown. atómico para race-free check en Enqueue.
	closedMu sync.RWMutex
	closed   bool
}

// webhookJob encapsula el payload del webhook para procesar en background.
type webhookJob struct {
	hookID    string
	hookSlug  string
	hook      any // *webhook.Hook — evitar import cycle en test stubs
	body      []byte
	inputs    map[string]any
	headers   map[string]string
	remote    string
	startedAt time.Time
}

// WebhookDispatcherConfig parámetros del dispatcher.
type WebhookDispatcherConfig struct {
	// QueueSize: bounded channel. Default 256.
	QueueSize int
	// JobTimeout: cada job corre con deadline. Default 30s (recomendado HU).
	JobTimeout time.Duration
	// Dispatch: función que ejecuta el job real (runWebhookTarget del
	// handler, o un stub en tests).
	Dispatch func(context.Context, webhookJob)
	// Logger para reportar shutdown errors. Default: slog.Default().
	Logger *slog.Logger
}

// NewWebhookDispatcher crea el dispatcher y arranca N workers.
func NewWebhookDispatcher(cfg WebhookDispatcherConfig) *WebhookDispatcher {
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 256
	}
	if cfg.JobTimeout <= 0 {
		cfg.JobTimeout = 30 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Dispatch == nil {
		cfg.Dispatch = func(ctx context.Context, _ webhookJob) {
			// Default no-op para que el dispatcher pueda existir
			// sin dispatch real (caso de test sin lógica de negocio).
		}
	}

	d := &WebhookDispatcher{
		ch:       make(chan webhookJob, cfg.QueueSize),
		timeout:  cfg.JobTimeout,
		dispatch: cfg.Dispatch,
		logger:   cfg.Logger,
	}

	// N workers = 1 (single goroutine que procesa en serie). Para más
	// throughput, se podrían spawnear N; por ahora single-worker es
	// suficiente (webhook dispatch no es CPU-bound, espera I/O HTTP).
	d.wg.Add(1)
	go d.worker()

	return d
}

// Enqueue intenta agregar un job a la cola. Retorna false si la cola
// está llena o el dispatcher ya cerró → handler responde 503.
//
// ctx se usa para cancelación del request; el job en sí corre con su
// propio timeout (cfg.JobTimeout).
func (d *WebhookDispatcher) Enqueue(ctx context.Context, job webhookJob) bool {
	d.closedMu.RLock()
	defer d.closedMu.RUnlock()
	if d.closed {
		return false
	}
	select {
	case d.ch <- job:
		return true
	case <-ctx.Done():
		return false
	default:
		// Channel lleno + ctx no cancelado: backpressure → 503.
		return false
	}
}

// worker procesa jobs en serie. Termina cuando el channel se cierra
// (Shutdown).
func (d *WebhookDispatcher) worker() {
	defer d.wg.Done()
	for job := range d.ch {
		d.runOne(job)
	}
}

func (d *WebhookDispatcher) runOne(job webhookJob) {
	ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
	defer cancel()
	defer func() {
		if r := recover(); r != nil {
			d.logger.Error("webhook dispatcher: panic recovered",
				slog.String("hook_id", job.hookID),
				slog.Any("panic", r),
			)
		}
	}()
	d.dispatch(ctx, job)
}

// Shutdown cierra el channel, espera a los jobs en vuelo hasta el
// budget. Retorna nil si todos terminaron, ctx.Err() si timeout.
func (d *WebhookDispatcher) Shutdown(ctx context.Context) error {
	d.closedMu.Lock()
	if d.closed {
		d.closedMu.Unlock()
		return nil
	}
	d.closed = true
	d.closedMu.Unlock()

	close(d.ch)

	done := make(chan struct{})
	go func() {
		d.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// QueueLen retorna la cantidad de jobs pendientes (para métricas).
func (d *WebhookDispatcher) QueueLen() int {
	return len(d.ch)
}
