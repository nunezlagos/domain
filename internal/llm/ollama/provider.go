// Package ollama — local Ollama provider (http://localhost:11434/api/chat).
package ollama

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/saargo/domain/internal/llm"
)

const (
	defaultBaseURL = "http://localhost:11434"
	defaultModel   = "llama3.1"
)

type Provider struct {
	BaseURL    string
	HTTPClient *http.Client
	Model      string
}

func New() *Provider {
	return &Provider{
		BaseURL:    defaultBaseURL,
		HTTPClient: &http.Client{Timeout: 10 * time.Minute},
		Model:      defaultModel,
	}
}

func (p *Provider) Name() string { return "ollama" }

type chatMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatReq struct {
	Model    string    `json:"model"`
	Messages []chatMsg `json:"messages"`
	Stream   bool      `json:"stream"`
	Options  map[string]any `json:"options,omitempty"`
}

type chatResp struct {
	Model     string  `json:"model"`
	Message   chatMsg `json:"message"`
	Done      bool    `json:"done"`
	PromptEvalCount int `json:"prompt_eval_count"`
	EvalCount       int `json:"eval_count"`
	DoneReason      string `json:"done_reason"`
}

func (p *Provider) buildRequest(opts llm.CompletionOptions, stream bool) chatReq {
	model := opts.Model
	if model == "" {
		model = p.Model
	}
	req := chatReq{Model: model, Stream: stream}
	if opts.SystemPrompt != "" {
		req.Messages = append(req.Messages, chatMsg{Role: "system", Content: opts.SystemPrompt})
	}
	for _, m := range opts.Messages {
		req.Messages = append(req.Messages, chatMsg{Role: m.Role, Content: m.Content})
	}
	if opts.Temperature > 0 || opts.MaxTokens > 0 {
		req.Options = map[string]any{}
		if opts.Temperature > 0 {
			req.Options["temperature"] = opts.Temperature
		}
		if opts.MaxTokens > 0 {
			req.Options["num_predict"] = opts.MaxTokens
		}
	}
	return req
}

func (p *Provider) Complete(ctx context.Context, opts llm.CompletionOptions) (*llm.Response, error) {
	body := p.buildRequest(opts, false)
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.BaseURL+"/api/chat", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama %d: %s", resp.StatusCode, string(msg))
	}
	var cr chatResp
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, err
	}
	return &llm.Response{
		Content:      cr.Message.Content,
		Model:        cr.Model,
		FinishReason: "stop",
		Usage: llm.Usage{
			PromptTokens:     cr.PromptEvalCount,
			CompletionTokens: cr.EvalCount,
			TotalTokens:      cr.PromptEvalCount + cr.EvalCount,
		},
	}, nil
}

func (p *Provider) CompleteStream(ctx context.Context, opts llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	body := p.buildRequest(opts, true)
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.BaseURL+"/api/chat", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama stream: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("ollama %d: %s", resp.StatusCode, string(msg))
	}
	out := make(chan llm.StreamChunk, 16)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		reader := bufio.NewReader(resp.Body)
		var usage llm.Usage
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if errors.Is(err, io.EOF) {
					out <- llm.StreamChunk{Done: true, Usage: &usage}
				}
				return
			}
			var ev chatResp
			if err := json.Unmarshal([]byte(line), &ev); err != nil {
				continue
			}
			if ev.Message.Content != "" {
				out <- llm.StreamChunk{Delta: ev.Message.Content}
			}
			if ev.Done {
				usage.PromptTokens = ev.PromptEvalCount
				usage.CompletionTokens = ev.EvalCount
				usage.TotalTokens = ev.PromptEvalCount + ev.EvalCount
				out <- llm.StreamChunk{Done: true, Usage: &usage}
				return
			}
		}
	}()
	return out, nil
}

// --- Embeddings ---

type Embedder struct {
	BaseURL    string
	Model      string // "nomic-embed-text", "mxbai-embed-large", etc.
	HTTPClient *http.Client
}

func NewEmbedder(model string) *Embedder {
	if model == "" {
		model = "nomic-embed-text"
	}
	return &Embedder{
		BaseURL:    defaultBaseURL,
		Model:      model,
		HTTPClient: &http.Client{Timeout: 1 * time.Minute},
	}
}

func (e *Embedder) Dimensions() int {
	// nomic-embed-text = 768 (truncate/pad a 1536 igual que voyage)
	// mxbai-embed-large = 1024
	// Asumimos 1536 final (padding zeros) para compat con observations schema.
	return 1536
}

type embedReq struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type embedResp struct {
	Embedding []float64 `json:"embedding"`
}

func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	body := embedReq{Model: e.Model, Prompt: text}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		e.BaseURL+"/api/embeddings", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama embed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama embed %d: %s", resp.StatusCode, string(msg))
	}
	var er embedResp
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		return nil, err
	}
	if len(er.Embedding) == 0 {
		return nil, errors.New("empty embedding")
	}
	out := make([]float32, e.Dimensions())
	for i, f := range er.Embedding {
		if i >= len(out) {
			break
		}
		out[i] = float32(f)
	}
	return out, nil
}
