// issue-07.4 — TokenBudgetManager: soft/hard limits con callback y modos
// error|truncate sobre el consumo de tokens de un run.
package tokens

import (
	"errors"
	"fmt"
	"sync"
)

// ErrBudgetTruncated señaliza corte graceful (modo truncate): el caller debe
// dejar de pedir tokens pero NO tratar el run como fallido.
var ErrBudgetTruncated = errors.New("token budget truncated")

// BudgetMode comportamiento al alcanzar el hard limit.
type BudgetMode string

const (
	ModeError    BudgetMode = "error"    // Track devuelve ErrBudgetExceeded
	ModeTruncate BudgetMode = "truncate" // Track devuelve ErrBudgetTruncated y marca Truncated
)

// TokenBudgetManager trackea consumo contra límites soft (warning) y hard.
type TokenBudgetManager struct {
	soft int
	hard int
	mode BudgetMode

	OnSoftLimit func(used, soft int)

	mu        sync.Mutex
	used      int
	softFired bool
	truncated bool
}

// BudgetState snapshot del estado.
type BudgetState struct {
	TokensUsed      int     `json:"tokens_used"`
	BudgetRemaining int     `json:"budget_remaining"`
	Percentage      float64 `json:"percentage"`
	Truncated       bool    `json:"truncated"`
}

// NewTokenBudget valida: hard > 0, hard ≤ modelMax (si modelMax > 0),
// soft ≤ hard (soft 0 = sin warning).
func NewTokenBudget(soft, hard, modelMax int, mode BudgetMode) (*TokenBudgetManager, error) {
	if hard <= 0 {
		return nil, fmt.Errorf("token budget: hard limit must be > 0, got %d", hard)
	}
	if modelMax > 0 && hard > modelMax {
		return nil, fmt.Errorf("token budget: hard %d exceeds model max_tokens %d", hard, modelMax)
	}
	if soft < 0 || soft > hard {
		return nil, fmt.Errorf("token budget: soft %d must be in [0, hard=%d]", soft, hard)
	}
	if mode == "" {
		mode = ModeError
	}
	if mode != ModeError && mode != ModeTruncate {
		return nil, fmt.Errorf("token budget: mode %q not valid", mode)
	}
	return &TokenBudgetManager{soft: soft, hard: hard, mode: mode}, nil
}

// Check pre-llamada: error si el budget ya está agotado.
func (m *TokenBudgetManager) Check() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.statusLocked()
}

func (m *TokenBudgetManager) statusLocked() error {
	if m.used < m.hard {
		return nil
	}
	if m.mode == ModeTruncate {
		m.truncated = true
		return ErrBudgetTruncated
	}
	return ErrBudgetExceeded
}

// Track suma n tokens. Dispara el soft callback una sola vez al cruzar el
// soft limit y devuelve el error de hard limit según el modo.
func (m *TokenBudgetManager) Track(n int) error {
	if n < 0 {
		n = 0
	}
	m.mu.Lock()
	m.used += n
	fireSoft := m.soft > 0 && !m.softFired && m.used >= m.soft
	if fireSoft {
		m.softFired = true
	}
	used := m.used
	err := m.statusLocked()
	cb := m.OnSoftLimit
	m.mu.Unlock()

	if fireSoft && cb != nil {
		cb(used, m.soft)
	}
	return err
}

// Reset vuelve el contador a cero (nuevo intento / nueva fase).
func (m *TokenBudgetManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.used = 0
	m.softFired = false
	m.truncated = false
}

// State retorna el snapshot actual.
func (m *TokenBudgetManager) State() BudgetState {
	m.mu.Lock()
	defer m.mu.Unlock()
	remaining := m.hard - m.used
	if remaining < 0 {
		remaining = 0
	}
	return BudgetState{
		TokensUsed:      m.used,
		BudgetRemaining: remaining,
		Percentage:      float64(m.used) / float64(m.hard) * 100,
		Truncated:       m.truncated,
	}
}
