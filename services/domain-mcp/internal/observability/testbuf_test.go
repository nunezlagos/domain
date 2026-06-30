package observability

import (
	"bytes"
	"log/slog"
	"sync"
)

// threadSafeBuffer wraps bytes.Buffer with a Mutex para tests que usan
// producers concurrentes (goroutines que llaman Enter/Log/etc) + workers
// que escriben logs al buffer via slog. Sin Mutex, race en bytes.Buffer.
type threadSafeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *threadSafeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *threadSafeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func newCapture() (*slog.Logger, *threadSafeBuffer) {
	tb := &threadSafeBuffer{}
	return slog.New(slog.NewJSONHandler(tb, &slog.HandlerOptions{Level: slog.LevelDebug})), tb
}
