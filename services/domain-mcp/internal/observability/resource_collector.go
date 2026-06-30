// Package observability: este archivo cubre el dominio resource tracking.
// Una goroutine samplea runtime.MemStats cada N segundos y persiste.
//
// issue-53.6 comprehensive.
package observability

import (
	"context"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ResourceSample es una lectura puntual de runtime.MemStats + goroutines.
type ResourceSample struct {
	Goroutines     int
	HeapAllocBytes int64
	HeapSysBytes   int64
	NumGC          int
	NumCPU         int
	CapturedAt     time.Time
}

// ResourceStore abstrae la persistencia.
type ResourceStore interface {
	InsertResourceSample(ctx context.Context, s ResourceSample) error
}

// PGResourceStore persiste en resource_samples.
type PGResourceStore struct {
	Pool *pgxpool.Pool
}

// InsertResourceSample ejecuta el INSERT.
func (s *PGResourceStore) InsertResourceSample(ctx context.Context, sample ResourceSample) error {
	_, err := s.Pool.Exec(ctx, `
		INSERT INTO resource_samples (goroutines, heap_alloc_bytes, heap_sys_bytes, num_gc, num_cpu, captured_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`,
		sample.Goroutines, sample.HeapAllocBytes, sample.HeapSysBytes, sample.NumGC, sample.NumCPU, sample.CapturedAt,
	)
	return err
}

// ResourceCollector arranca una goroutine que samplea runtime.MemStats
// cada interval. Stop es idempotente y bloquea hasta el flush final.
type ResourceCollector struct {
	store    ResourceStore
	logger   *slog.Logger
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
	once     sync.Once
}

// IntervalDefaultSeconds es el intervalo por default entre samples.
const IntervalDefaultSeconds = 30

// NewResourceCollector retorna un collector listo. Llamar Start para arrancar.
func NewResourceCollector(store ResourceStore, logger *slog.Logger, intervalSeconds int) *ResourceCollector {
	if intervalSeconds <= 0 {
		intervalSeconds = IntervalDefaultSeconds
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &ResourceCollector{
		store:    store,
		logger:   logger,
		interval: time.Duration(intervalSeconds) * time.Second,
		stopCh:   make(chan struct{}),
	}
}

// Start arranca la goroutine de sampling.
func (c *ResourceCollector) Start() {
	c.once.Do(func() {
		c.wg.Add(1)
		go c.run()
	})
}

// Stop cierra el canal y espera. Idempotente.
func (c *ResourceCollector) Stop() {
	select {
	case <-c.stopCh:
		return
	default:
		close(c.stopCh)
	}
	c.wg.Wait()
}

func (c *ResourceCollector) run() {
	defer c.wg.Done()
	t := time.NewTicker(c.interval)
	defer t.Stop()

	c.tick()
	for {
		select {
		case <-c.stopCh:
			return
		case <-t.C:
			c.tick()
		}
	}
}

// tick captura una sample y la persiste.
func (c *ResourceCollector) tick() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	sample := ResourceSample{
		Goroutines:     runtime.NumGoroutine(),
		HeapAllocBytes: int64(m.HeapAlloc),
		HeapSysBytes:   int64(m.HeapSys),
		NumGC:          int(m.NumGC),
		NumCPU:         runtime.NumCPU(),
		CapturedAt:     time.Now(),
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	if err := c.store.InsertResourceSample(ctx, sample); err != nil {
		c.logger.Warn("resource sample persist failed",
			slog.String("error", err.Error()))
	}
}

// Capture retorna la sample actual (util para tests o uso ad-hoc).
func (c *ResourceCollector) Capture() ResourceSample {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return ResourceSample{
		Goroutines:     runtime.NumGoroutine(),
		HeapAllocBytes: int64(m.HeapAlloc),
		HeapSysBytes:   int64(m.HeapSys),
		NumGC:          int(m.NumGC),
		NumCPU:         runtime.NumCPU(),
		CapturedAt:     time.Now(),
	}
}

var _ ResourceStore = (*PGResourceStore)(nil)
var _ *pgxpool.Pool = nil
