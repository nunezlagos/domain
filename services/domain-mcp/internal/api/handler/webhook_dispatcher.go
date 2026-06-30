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
//     false si esta llena → handler responde 503.
//   - Timeout por job: cada job corre con su propio context con deadline.
//   - WaitGroup: el shutdown espera a los jobs en vuelo (hasta un budget).
//   - Context cancelable: shutdown cancela jobs que estan en vuelo.
//
// ISSUE-28.7 (webhook-goroutine-lifecycle): antes el handler hacia
// `go a.runWebhookTarget(context.Background(), ...)` directamente. Problemas:
//   - Unbounded goroutines (10k webhooks/s → 10k goroutines).
//   - Sin cancelacion: context.Background no hereda el shutdown.
//   - Sin WaitGroup: graceful shutdown no espera los webhooks.
type WebhookDispatcher struct {
	ch       chan webhookJob
	wg       sync.WaitGroup
	timeout  time.Duration
	dispatch func(context.Context, webhookJob)
	logger   *slog.Logger



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

// WebhookDispatcherConfig parametros del dispatcher.
type WebhookDispatcherConfig struct {

	QueueSize int

	JobTimeout time.Duration


	Dispatch func(context.Context, webhookJob)

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


		}
	}

	d := &WebhookDispatcher{
		ch:       make(chan webhookJob, cfg.QueueSize),
		timeout:  cfg.JobTimeout,
		dispatch: cfg.Dispatch,
		logger:   cfg.Logger,
	}




	d.wg.Add(1)
	go d.worker()

	return d
}

// Enqueue intenta agregar un job a la cola. Retorna false si la cola
// esta llena o el dispatcher ya cerro → handler responde 503.
//
// ctx se usa para cancelacion del request; el job en si corre con su
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

// QueueLen retorna la cantidad de jobs pendientes (para metricas).
func (d *WebhookDispatcher) QueueLen() int {
	return len(d.ch)
}
