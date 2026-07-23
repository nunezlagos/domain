// Probe de la Fase B: ejercita el bridge ACP contra opencode REAL con los
// modelos FREE de opencode (sin auth), midiendo el costo real (boot de opencode
// + inferencia) y la fiabilidad por modelo. La selección de modelo se hace por
// config-file de opencode resuelto vía HOME (el bridge ya pasa HOME por su
// allowlist), que es el mecanismo del futuro rolling-model.
//
// Uso: acp-probe [modelo]   (sin arg = itera los 6 free; con arg = solo ese)
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	acpbridge "nunezlagos/domain/internal/agentbridge/acp"
)

var logger = slog.New(slog.NewTextHandler(io.Discard, nil))

var freeModels = []string{
	"opencode/big-pickle",
	"opencode/deepseek-v4-flash-free",
	"opencode/laguna-s-2.1-free",
	"opencode/mimo-v2.5-free",
	"opencode/nemotron-3-ultra-free",
	"opencode/north-mini-code-free",
}

// prepareHome crea un HOME aislado con un opencode.json que fija el modelo, y lo
// activa como HOME del proceso (el bridge lo propaga al subproceso opencode).
func prepareHome(base, model string) error {
	dir := filepath.Join(base, "home-"+filepath.Base(model))
	cfgDir := filepath.Join(dir, ".config", "opencode")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return err
	}
	cfg := fmt.Sprintf(`{"$schema":"https://opencode.ai/config.json","model":%q}`, model)
	if err := os.WriteFile(filepath.Join(cfgDir, "opencode.json"), []byte(cfg), 0o644); err != nil {
		return err
	}
	return os.Setenv("HOME", dir)
}

func call(prompt string, timeout time.Duration) (string, time.Duration, error) {
	start := time.Now()
	p, err := acpbridge.Spawn(context.Background(), acpbridge.Config{
		Bin: "opencode", Args: []string{"acp"}, Timeout: timeout,
	}, logger)
	if err != nil {
		return "", time.Since(start), err
	}
	defer func() { _ = p.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := p.Prompt(ctx, prompt)
	return out, time.Since(start), err
}

func probeModel(base, model string, to time.Duration) {
	if err := prepareHome(base, model); err != nil {
		fmt.Printf("\n### %s — setup FALLó: %v\n", model, err)
		return
	}
	fmt.Printf("\n### %s\n", model)

	out, d, err := call("responde solo con la palabra: pong", to)
	if err != nil {
		fmt.Printf("  funcional: FALLó %s [%s]\n", err, d.Round(time.Millisecond))
		return
	}
	fmt.Printf("  funcional: OK %s · resp=%q\n", d.Round(time.Millisecond), trim(out, 60))

	var tot time.Duration
	n := 3
	for i := 0; i < n; i++ {
		_, d, err := call("di 'ok' y nada mas", to)
		tot += d
		if err != nil {
			fmt.Printf("  seq #%d ERR %s\n", i+1, err)
		}
	}
	fmt.Printf("  seq avg (boot+inferencia) = %s\n", (tot / time.Duration(n)).Round(time.Millisecond))

	ok, errs := conc(2, 4, to)
	fmt.Printf("  conc c=2 n=4: ok=%d err=%d (rate-limit del free tier)\n", ok, errs)
}

func conc(concurrency, calls int, to time.Duration) (int, int) {
	errsCh := make([]error, calls)
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	for i := 0; i < calls; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			_, _, err := call("di 'ok' y nada mas", to)
			errsCh[i] = err
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, e := range errsCh {
		if e == nil {
			ok++
		}
	}
	return ok, calls - ok
}

func main() {
	const to = 120 * time.Second
	base, err := os.MkdirTemp("", "acp-probe-homes")
	if err != nil {
		panic(err)
	}
	models := freeModels
	if len(os.Args) > 1 {
		models = []string{os.Args[1]}
	}
	fmt.Printf("=== FASE B: opencode real, %d modelo(s) free, sin auth ===\n", len(models))
	for _, m := range models {
		probeModel(base, m, to)
	}
}

func trim(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
