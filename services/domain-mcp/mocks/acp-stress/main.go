// Harness de estrés del bridge ACP (internal/agentbridge/acp) contra el mock
// agent. Ejercita el MISMO camino que usa el sistema (Spawn→Prompt→Close, igual
// que llm/acp.Provider.Complete) bajo ramp + spike + modos de fallo, y mide
// latencias, error-rate, throughput y leak de goroutines.
//
// Uso: acp-stress <ruta-al-mock-agent-bin>
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime"
	"sync"
	"time"

	acpbridge "nunezlagos/domain/internal/agentbridge/acp"
)

var logger = slog.New(slog.NewTextHandler(io.Discard, nil))

func cfg(bin string, env ...string) acpbridge.Config {
	return acpbridge.Config{Bin: bin, Env: env, Timeout: 30 * time.Second}
}

// oneCall corre un ciclo completo spawn→prompt→close y mide su latencia.
func oneCall(c acpbridge.Config, promptTimeout time.Duration) result {
	start := time.Now()
	p, err := acpbridge.Spawn(context.Background(), c, logger)
	if err != nil {
		return result{time.Since(start), err}
	}
	defer func() { _ = p.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), promptTimeout)
	defer cancel()
	_, err = p.Prompt(ctx, "ping")
	return result{time.Since(start), err}
}

func runBatch(name string, concurrency, calls int, c acpbridge.Config, promptTimeout time.Duration) {
	results := make([]result, calls)
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	wall := time.Now()
	for i := 0; i < calls; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			results[i] = oneCall(c, promptTimeout)
		}(i)
	}
	wg.Wait()
	summarize(name, results, time.Since(wall))
}

func goroutines() int {
	runtime.GC()
	time.Sleep(300 * time.Millisecond)
	return runtime.NumGoroutine()
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("uso: acp-stress <ruta-al-mock-agent-bin>")
		os.Exit(2)
	}
	bin := os.Args[1]
	base := goroutines()
	fmt.Printf("baseline goroutines=%d agent=%s\n", base, bin)

	runBatch("warmup", 1, 1, cfg(bin), 10*time.Second)

	fmt.Println("\n=== RAMP (concurrencia creciente) ===")
	for _, lvl := range []int{1, 10, 50, 100, 200} {
		runBatch(fmt.Sprintf("ramp c=%d", lvl), lvl, lvl*5, cfg(bin, "MOCK_CHUNKS=5"), 15*time.Second)
	}

	fmt.Println("\n=== SPIKE (0→200 súbito) ===")
	runBatch("spike c=200", 200, 400, cfg(bin, "MOCK_CHUNKS=5"), 20*time.Second)

	fmt.Println("\n=== STREAMING (50 chunks, 2ms c/u) ===")
	runBatch("stream", 50, 100, cfg(bin, "MOCK_CHUNKS=50", "MOCK_CHUNK_DELAY_MS=2"), 20*time.Second)

	fmt.Println("\n=== MODOS DE FALLO (buscar puntos débiles) ===")
	runBatch("crash-init", 20, 40, cfg(bin, "MOCK_FAIL=crash-init"), 10*time.Second)
	runBatch("crash-prompt", 20, 40, cfg(bin, "MOCK_FAIL=crash-prompt"), 10*time.Second)
	runBatch("hang+timeout2s", 20, 40, cfg(bin, "MOCK_FAIL=hang"), 2*time.Second)
	runBatch("flood-chunks", 10, 20, cfg(bin, "MOCK_FAIL=flood"), 30*time.Second)

	after := goroutines()
	fmt.Printf("\n=== LEAK CHECK ===\ngoroutines baseline=%d final=%d delta=%d\n", base, after, after-base)
	if after-base > 20 {
		fmt.Println("⚠️  posible leak de goroutines")
	} else {
		fmt.Println("✅ sin leak significativo de goroutines")
	}
}
