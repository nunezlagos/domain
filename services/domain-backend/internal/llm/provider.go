// Package llm — issue-06.1 Provider interface + factory thread-safe.
//
// Provider abstrae completion + streaming + embeddings sobre cualquier LLM
// (OpenAI, Anthropic, Google, Ollama, ...). El factory permite swap por env
// var DOMAIN_LLM_PROVIDER y registro de providers custom.
//
// Embedder ya estaba definido (Nop/Fake); aquí extendemos con Provider
// completo. Cada Provider debe satisfacer Provider Y Embedder.

package llm

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// CompletionOptions params para Complete/CompleteStream.
type CompletionOptions struct {
	Model       string  // "gpt-4o", "claude-sonnet-4-6", etc.
	Temperature float64 // 0.0..2.0
	MaxTokens   int     // 0 = unlimited (provider-default)
	SystemPrompt string
	Messages    []Message // history conversation
	StopSequences []string
	Tools       []ToolDef // function calling (issue-08.2)
}

// Message en conversation history.
type Message struct {
	Role    string `json:"role"`    // "user" | "assistant" | "system" | "tool"
	Content string `json:"content"`
	// ToolCalls cuando el assistant pidió ejecutar tools en la respuesta previa.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
	// ToolCallID cuando este mensaje ES la respuesta de un tool.
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// ToolDef describe una tool disponible para el modelo.
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"parameters"`
}

// ToolCall es una invocación que el modelo pidió hacer.
type ToolCall struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// Usage tokens consumidos en una request.
type Usage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	CostUSD          float64 `json:"cost_usd,omitempty"`
}

// Response de Complete.
type Response struct {
	Content      string     `json:"content"`
	Model        string     `json:"model"`
	Usage        Usage      `json:"usage"`
	FinishReason string     `json:"finish_reason"` // "stop" | "length" | "tool_use"
	ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
}

// StreamChunk delta de CompleteStream.
type StreamChunk struct {
	Delta string `json:"delta"`
	Done  bool   `json:"done"`
	// Error se setea cuando el provider aborta mid-stream (ej: timeout
	// parcial, conexión cerrada). El chunk con Error=true debe tener
	// Done=true también (es el último chunk). ISSUE-28.6: el circuit
	// breaker usa este flag para detectar errores mid-stream y abrir.
	Error string `json:"error,omitempty"`
	// Usage solo en el último chunk (Done=true).
	Usage *Usage `json:"usage,omitempty"`
}

// Provider es la interfaz que todo LLM provider debe satisfacer.
type Provider interface {
	Name() string
	Complete(ctx context.Context, opts CompletionOptions) (*Response, error)
	CompleteStream(ctx context.Context, opts CompletionOptions) (<-chan StreamChunk, error)
}

// Factory thread-safe registry de providers + Embedders.
type Factory struct {
	mu        sync.RWMutex
	providers map[string]Provider
	embedders map[string]Embedder
	defaultProvider string
	defaultEmbedder string
}

// NewFactory crea factory vacío.
func NewFactory() *Factory {
	return &Factory{
		providers: map[string]Provider{},
		embedders: map[string]Embedder{},
	}
}

// Register agrega o reemplaza un provider por nombre.
func (f *Factory) Register(name string, p Provider) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.providers[name] = p
}

// RegisterEmbedder agrega Embedder por nombre.
func (f *Factory) RegisterEmbedder(name string, e Embedder) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.embedders[name] = e
}

// SetDefault setea el nombre del provider/embedder default.
func (f *Factory) SetDefault(provider, embedder string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if provider != "" {
		f.defaultProvider = provider
	}
	if embedder != "" {
		f.defaultEmbedder = embedder
	}
}

// Get retorna el provider por nombre.
func (f *Factory) Get(name string) (Provider, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	p, ok := f.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", name)
	}
	return p, nil
}

// GetEmbedder retorna el embedder por nombre.
func (f *Factory) GetEmbedder(name string) (Embedder, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	e, ok := f.embedders[name]
	if !ok {
		return nil, fmt.Errorf("embedder not found: %s", name)
	}
	return e, nil
}

// GetDefault retorna el provider default seteado via SetDefault.
func (f *Factory) GetDefault() (Provider, error) {
	f.mu.RLock()
	def := f.defaultProvider
	f.mu.RUnlock()
	if def == "" {
		return nil, errors.New("no default provider set")
	}
	return f.Get(def)
}

// GetDefaultEmbedder retorna el embedder default.
func (f *Factory) GetDefaultEmbedder() (Embedder, error) {
	f.mu.RLock()
	def := f.defaultEmbedder
	f.mu.RUnlock()
	if def == "" {
		return nil, errors.New("no default embedder set")
	}
	return f.GetEmbedder(def)
}

// List devuelve names registrados (orden no garantizado).
func (f *Factory) List() []string {
	f.mu.RLock()
	defer f.mu.RUnlock()
	names := make([]string, 0, len(f.providers))
	for n := range f.providers {
		names = append(names, n)
	}
	return names
}
