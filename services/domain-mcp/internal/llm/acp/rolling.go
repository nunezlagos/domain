package acp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	defaultCooldown   = 60 * time.Second
	defaultRefreshTTL = time.Hour
)

// roller mantiene el roster de modelos free de opencode y rota round-robin con
// cooldown por modelo. El roster se descubre dinamicamente (no hardcodeado).
type roller struct {
	mu       sync.Mutex
	models   []string
	idx      int
	cooldown map[string]time.Time
	homes    map[string]string
	cdTTL    time.Duration
	baseHome string
	discover func(context.Context) ([]string, error)
	now      func() time.Time
}

func newRoller(baseHome string, cdTTL time.Duration, discover func(context.Context) ([]string, error)) *roller {
	return &roller{
		cooldown: map[string]time.Time{},
		homes:    map[string]string{},
		cdTTL:    cdTTL,
		baseHome: baseHome,
		discover: discover,
		now:      time.Now,
	}
}

// setRoster reemplaza el roster. Si models esta vacio conserva el anterior
// (fallback al ultimo roster conocido).
func (r *roller) setRoster(models []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(models) == 0 {
		return
	}
	r.models = models
	if r.idx >= len(models) {
		r.idx = 0
	}
	for _, m := range models {
		if _, ok := r.homes[m]; ok {
			continue
		}
		if h, err := prepareModelHome(r.baseHome, m); err == nil {
			r.homes[m] = h
		}
	}
}

// next devuelve el siguiente modelo sano (round-robin) y su HOME. Si todos
// estan en cooldown devuelve el siguiente igual (best-effort).
func (r *roller) next() (model, home string, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := len(r.models)
	if n == 0 {
		return "", "", false
	}
	now := r.now()
	for i := 0; i < n; i++ {
		m := r.models[r.idx]
		r.idx = (r.idx + 1) % n
		if until, cd := r.cooldown[m]; cd && now.Before(until) {
			continue
		}
		return m, r.homes[m], true
	}
	m := r.models[r.idx]
	r.idx = (r.idx + 1) % n
	return m, r.homes[m], true
}

func (r *roller) cooldownModel(model string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cooldown[model] = r.now().Add(r.cdTTL)
}

func (r *roller) size() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.models)
}

// refreshNow descubre el roster una vez; conserva el anterior si falla.
func (r *roller) refreshNow(ctx context.Context) {
	models, err := r.discover(ctx)
	if err != nil {
		return
	}
	r.setRoster(models)
}

// refreshLoop re-descubre el roster cada ttl hasta que ctx se cancele.
func (r *roller) refreshLoop(ctx context.Context, ttl time.Duration) {
	t := time.NewTicker(ttl)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			r.refreshNow(ctx)
		}
	}
}

// prepareModelHome crea un HOME aislado con un opencode.json que fija el modelo.
func prepareModelHome(base, model string) (string, error) {
	dir := filepath.Join(base, "home-"+strings.ReplaceAll(model, "/", "_"))
	cfgDir := filepath.Join(dir, ".config", "opencode")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return "", err
	}
	cfg := `{"$schema":"https://opencode.ai/config.json","model":"` + model + `"}`
	if err := os.WriteFile(filepath.Join(cfgDir, "opencode.json"), []byte(cfg), 0o644); err != nil {
		return "", err
	}
	return dir, nil
}
