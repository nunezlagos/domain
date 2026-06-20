package handler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestWebhookDispatcher_EnqueueAccepts: encolar un job retorna true y se ejecuta.
func TestWebhookDispatcher_EnqueueAccepts(t *testing.T) {
	var ran int32
	d := NewWebhookDispatcher(WebhookDispatcherConfig{
		QueueSize:  10,
		JobTimeout: 100 * time.Millisecond,
		Dispatch: func(ctx context.Context, _ webhookJob) {
			atomic.AddInt32(&ran, 1)
		},
	})
	defer d.Shutdown(context.Background())

	ok := d.Enqueue(context.Background(), webhookJob{hookID: "h1"})
	require.True(t, ok)

	// Esperar a que el worker procese el job.
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&ran) == 1
	}, 500*time.Millisecond, 10*time.Millisecond)
}

// TestWebhookDispatcher_Backpressure: cola llena → Enqueue retorna false.
func TestWebhookDispatcher_Backpressure(t *testing.T) {
	blockCh := make(chan struct{})
	defer close(blockCh)
	d := NewWebhookDispatcher(WebhookDispatcherConfig{
		QueueSize: 2, // solo 2 slots
		JobTimeout: 5 * time.Second,
		Dispatch: func(ctx context.Context, _ webhookJob) {
			<-blockCh // bloquea hasta que cerremos el test
		},
	})
	defer d.Shutdown(context.Background())

	// Llenar la cola + 1 worker ocupado.
	require.True(t, d.Enqueue(context.Background(), webhookJob{hookID: "1"}))
	require.True(t, d.Enqueue(context.Background(), webhookJob{hookID: "2"}))
	// worker toma 1 → quedan 1 slot libre. Enqueue #3 OK (no lleno aún).
	require.True(t, d.Enqueue(context.Background(), webhookJob{hookID: "3"}))
	// Pero el worker está bloqueado, no consume más. Enqueue #4 debe
	// fallar (default branch → false).
	require.False(t, d.Enqueue(context.Background(), webhookJob{hookID: "4"}),
		"cola llena → backpressure")
}

// TestWebhookDispatcher_JobTimeout: job que excede timeout es cancelado.
func TestWebhookDispatcher_JobTimeout(t *testing.T) {
	started := make(chan struct{}, 1)
	cancelled := make(chan struct{}, 1)
	d := NewWebhookDispatcher(WebhookDispatcherConfig{
		QueueSize:  1,
		JobTimeout: 50 * time.Millisecond,
		Dispatch: func(ctx context.Context, _ webhookJob) {
			select {
			case started <- struct{}{}:
			default:
			}
			<-ctx.Done()
			select {
			case cancelled <- struct{}{}:
			default:
			}
		},
	})
	defer d.Shutdown(context.Background())

	require.True(t, d.Enqueue(context.Background(), webhookJob{}))
	require.Eventually(t, func() bool {
		select {
		case <-started:
			return true
		default:
			return false
		}
	}, 200*time.Millisecond, 5*time.Millisecond)
	require.Eventually(t, func() bool {
		select {
		case <-cancelled:
			return true
		default:
			return false
		}
	}, 500*time.Millisecond, 10*time.Millisecond, "ctx con timeout debe cancelar el job")
}

// TestWebhookDispatcher_ShutdownWaitsJobs: shutdown espera a los jobs
// en vuelo (hasta el budget).
func TestWebhookDispatcher_ShutdownWaitsJobs(t *testing.T) {
	var finished int32
	d := NewWebhookDispatcher(WebhookDispatcherConfig{
		QueueSize:  3,
		JobTimeout: 1 * time.Second,
		Dispatch: func(ctx context.Context, _ webhookJob) {
			time.Sleep(100 * time.Millisecond)
			atomic.AddInt32(&finished, 1)
		},
	})

	// Encolar 3 jobs.
	require.True(t, d.Enqueue(context.Background(), webhookJob{}))
	require.True(t, d.Enqueue(context.Background(), webhookJob{}))
	require.True(t, d.Enqueue(context.Background(), webhookJob{}))

	// Shutdown con budget generoso: espera a que los 3 terminen.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, d.Shutdown(ctx))
	require.Equal(t, int32(3), atomic.LoadInt32(&finished), "shutdown debe esperar jobs en vuelo")
}

// TestWebhookDispatcher_ShutdownRejectsNew: tras Shutdown, Enqueue rechaza.
func TestWebhookDispatcher_ShutdownRejectsNew(t *testing.T) {
	d := NewWebhookDispatcher(WebhookDispatcherConfig{
		QueueSize:  1,
		JobTimeout: 100 * time.Millisecond,
		Dispatch:   func(ctx context.Context, _ webhookJob) {},
	})

	require.NoError(t, d.Shutdown(context.Background()))
	require.False(t, d.Enqueue(context.Background(), webhookJob{}),
		"post-shutdown Enqueue debe rechazar")
}

// TestWebhookDispatcher_PanicRecovered: un job que panicea NO mata el worker.
func TestWebhookDispatcher_PanicRecovered(t *testing.T) {
	var afterPanic int32
	d := NewWebhookDispatcher(WebhookDispatcherConfig{
		QueueSize:  3,
		JobTimeout: 100 * time.Millisecond,
		Dispatch: func(ctx context.Context, job webhookJob) {
			if job.hookID == "boom" {
				panic("kaboom")
			}
			atomic.AddInt32(&afterPanic, 1)
		},
	})
	defer d.Shutdown(context.Background())

	require.True(t, d.Enqueue(context.Background(), webhookJob{hookID: "boom"}))
	require.True(t, d.Enqueue(context.Background(), webhookJob{hookID: "ok1"}))
	require.True(t, d.Enqueue(context.Background(), webhookJob{hookID: "ok2"}))

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&afterPanic) == 2
	}, 500*time.Millisecond, 10*time.Millisecond,
		"tras panic, el worker sigue procesando (recover)")
}
