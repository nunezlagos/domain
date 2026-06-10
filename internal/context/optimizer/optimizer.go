// issue-07.1 context-optimizer — comprime context para LLM calls cuando excede
// budget. Estrategias: oldest-first eviction, summary collapsing, token budget.
package optimizer

import (
	"sort"
	"strings"
	"time"
)

// Message es una unidad de context (observation, prompt, session line).
type Message struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"`         // system | user | assistant | tool
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	Pinned    bool      `json:"pinned,omitempty"`  // No-evict: system + user instructions críticos
	Tokens    int       `json:"tokens"`            // pre-calculado por tokens.Estimate
}

// Config controla el optimizer.
type Config struct {
	MaxTokens int     // budget total (mandatorio)
	SummaryFn func(messages []Message) string // si no nil, sumariza mensajes evictados
	KeepLast  int     // siempre conserva los N más recientes (default 6)
}

// Result es el output del optimizer.
type Result struct {
	Kept    []Message `json:"kept"`
	Evicted int       `json:"evicted_count"`
	Summary string    `json:"summary,omitempty"`
	Tokens  int       `json:"final_tokens"`
}

// Optimize aplica eviction + opcional summarization para fit en MaxTokens.
//
// Algoritmo:
//  1. Sort cronológico (oldest first).
//  2. Conserva siempre Pinned + últimos KeepLast.
//  3. Si total > MaxTokens, evict desde el oldest hasta caber.
//  4. Si SummaryFn provisto, genera summary de los evicted y lo agrega como
//     mensaje role=system al inicio (cuenta como tokens propios).
func Optimize(messages []Message, cfg Config) Result {
	if cfg.MaxTokens <= 0 {
		return Result{Kept: messages, Tokens: sumTokens(messages)}
	}
	if cfg.KeepLast <= 0 {
		cfg.KeepLast = 6
	}

	// Copy + sort cronológico ASC
	sorted := make([]Message, len(messages))
	copy(sorted, messages)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.Before(sorted[j].CreatedAt)
	})

	total := sumTokens(sorted)
	if total <= cfg.MaxTokens {
		return Result{Kept: sorted, Tokens: total}
	}

	// Mark evictables: NOT pinned AND NOT en los últimos KeepLast.
	keepLastStart := len(sorted) - cfg.KeepLast
	if keepLastStart < 0 {
		keepLastStart = 0
	}

	evictable := make([]int, 0, len(sorted))
	for i, m := range sorted {
		if m.Pinned {
			continue
		}
		if i >= keepLastStart {
			continue
		}
		evictable = append(evictable, i)
	}

	kept := make([]bool, len(sorted))
	for i := range kept {
		kept[i] = true
	}
	evicted := []Message{}

	// Evict desde el oldest evictable hasta caber.
	for _, idx := range evictable {
		if total <= cfg.MaxTokens {
			break
		}
		kept[idx] = false
		evicted = append(evicted, sorted[idx])
		total -= sorted[idx].Tokens
	}

	// Build result kept en orden cronológico.
	out := make([]Message, 0, len(sorted))
	for i, m := range sorted {
		if kept[i] {
			out = append(out, m)
		}
	}

	summary := ""
	if cfg.SummaryFn != nil && len(evicted) > 0 {
		summary = cfg.SummaryFn(evicted)
		if summary != "" {
			out = append([]Message{{
				ID:        "summary-" + time.Now().Format("20060102T150405"),
				Role:      "system",
				Content:   "Resumen de " + countStr(len(evicted)) + " mensajes evictados:\n" + summary,
				CreatedAt: sorted[0].CreatedAt,
				Tokens:    estimateTokens(summary) + 16,
			}}, out...)
			total += estimateTokens(summary) + 16
		}
	}

	return Result{
		Kept:    out,
		Evicted: len(evicted),
		Summary: summary,
		Tokens:  total,
	}
}

func sumTokens(msgs []Message) int {
	s := 0
	for _, m := range msgs {
		s += m.Tokens
	}
	return s
}

func estimateTokens(text string) int {
	// 4 chars per token aprox
	return (len(text) + 3) / 4
}

func countStr(n int) string {
	switch n {
	case 0:
		return "0"
	case 1:
		return "1"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}

// SimpleSummary es un summary fn de fallback: concatena primeras 60 chars
// de cada mensaje evictado, prefijo por role. Sin LLM.
func SimpleSummary(msgs []Message) string {
	var lines []string
	for _, m := range msgs {
		c := strings.TrimSpace(m.Content)
		if len(c) > 60 {
			c = c[:60] + "..."
		}
		lines = append(lines, "- ["+m.Role+"] "+c)
	}
	return strings.Join(lines, "\n")
}
