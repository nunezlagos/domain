// HU-06.6 token-count-stream — streaming token counter para responses LLM.
//
// Diseñado para ser consumido por wrappers de Anthropic/OpenAI/Ollama
// que streamean tokens. Calcula:
//   - Tokens emitidos hasta el momento (incremental).
//   - Tasa (tokens/s) sliding window 5s.
//   - Token budget enforcement: si excede limit, cancela el stream.
//
// Uso:
//
//	tc := tokens.NewStreamCounter(maxTokens, time.Second*5)
//	for chunk := range llmStream {
//	    if err := tc.Add(chunk.Text); err != nil { // budget exceeded
//	        return err  // contexto cancelado upstream
//	    }
//	    emit(chunk.Text)
//	}
//	final := tc.Snapshot()  // total + duration + avg rate

package tokens

import (
	"errors"
	"sync"
	"time"
)

// ErrBudgetExceeded se devuelve cuando Add sobrepasa el límite configurado.
var ErrBudgetExceeded = errors.New("token budget exceeded")

// StreamCounter es thread-safe — múltiples goroutines pueden llamar Add/Snapshot.
type StreamCounter struct {
	maxTokens int
	window    time.Duration

	mu       sync.Mutex
	startAt  time.Time
	total    int
	events   []event // sliding window
	clock    func() time.Time
}

type event struct {
	at     time.Time
	tokens int
}

// StreamSnapshot es la foto del estado del stream en un momento dado.
type StreamSnapshot struct {
	Total         int           `json:"total"`
	WindowTokens  int           `json:"window_tokens"`
	WindowSeconds float64       `json:"window_seconds"`
	RatePerSecond float64       `json:"rate_per_second"`
	Duration      time.Duration `json:"duration"`
	BudgetUsed    float64       `json:"budget_used"` // 0..1
}

// NewStreamCounter crea un counter con budget y ventana sliding.
// maxTokens=0 desactiva enforcement (budget = ∞).
// window default 5s si <=0.
func NewStreamCounter(maxTokens int, window time.Duration) *StreamCounter {
	if window <= 0 {
		window = 5 * time.Second
	}
	return &StreamCounter{
		maxTokens: maxTokens,
		window:    window,
		startAt:   time.Now(),
		clock:     time.Now,
	}
}

// withClock para tests determinísticos.
func (c *StreamCounter) withClock(f func() time.Time) *StreamCounter {
	c.clock = f
	c.startAt = f()
	return c
}

// Add agrega tokens estimados desde texto. Retorna ErrBudgetExceeded si pasa.
func (c *StreamCounter) Add(text string) error {
	return c.AddTokens(Estimate(text))
}

// AddTokens agrega un conteo exacto (cuando el provider devuelve token count).
func (c *StreamCounter) AddTokens(n int) error {
	if n <= 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	c.total += n
	if c.maxTokens > 0 && c.total > c.maxTokens {
		return ErrBudgetExceeded
	}
	c.events = append(c.events, event{at: c.clock(), tokens: n})
	c.pruneLocked()
	return nil
}

// Snapshot devuelve métricas actuales sin mutar estado.
func (c *StreamCounter) Snapshot() StreamSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pruneLocked()

	now := c.clock()
	dur := now.Sub(c.startAt)
	winT := 0
	for _, e := range c.events {
		winT += e.tokens
	}
	winSec := c.window.Seconds()
	rate := 0.0
	if winSec > 0 {
		rate = float64(winT) / winSec
	}
	used := 0.0
	if c.maxTokens > 0 {
		used = float64(c.total) / float64(c.maxTokens)
	}
	return StreamSnapshot{
		Total:         c.total,
		WindowTokens:  winT,
		WindowSeconds: winSec,
		RatePerSecond: rate,
		Duration:      dur,
		BudgetUsed:    used,
	}
}

// Total devuelve solo el total acumulado (lock-light).
func (c *StreamCounter) Total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.total
}

// Remaining devuelve budget restante o math.MaxInt si ilimitado.
func (c *StreamCounter) Remaining() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.maxTokens <= 0 {
		return int(^uint(0) >> 1)
	}
	r := c.maxTokens - c.total
	if r < 0 {
		return 0
	}
	return r
}

func (c *StreamCounter) pruneLocked() {
	cutoff := c.clock().Add(-c.window)
	i := 0
	for i < len(c.events) && c.events[i].at.Before(cutoff) {
		i++
	}
	c.events = c.events[i:]
}
