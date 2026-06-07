// Package openai — OpenAI Chat Completions + Embeddings provider.
//
// Usa /v1/chat/completions y /v1/embeddings de la API OpenAI. Sin deps.
package openai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/saargo/domain/internal/llm"
)

const (
	defaultBaseURL = "https://api.openai.com"
	defaultModel   = "gpt-4o"
	defaultEmbedModel = "text-embedding-3-small" // 1536 dims
)

type Provider struct {
	APIKey     string
	Org        string
	BaseURL    string
	HTTPClient *http.Client
	Model      string
}

func New(apiKey string) *Provider {
	return &Provider{
		APIKey:     apiKey,
		BaseURL:    defaultBaseURL,
		HTTPClient: &http.Client{Timeout: 5 * time.Minute},
		Model:      defaultModel,
	}
}

func (p *Provider) Name() string { return "openai" }

type chatRequest struct {
	Model       string         `json:"model"`
	Messages    []chatMessage  `json:"messages"`
	Temperature *float64       `json:"temperature,omitempty"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Stop        []string       `json:"stop,omitempty"`
	Tools       []chatTool     `json:"tools,omitempty"`
	Stream      bool           `json:"stream,omitempty"`
}

type chatMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Name       string         `json:"name,omitempty"`
}

type chatToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // "function"
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON string
	} `json:"function"`
}

type chatTool struct {
	Type     string `json:"type"` // "function"
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description,omitempty"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

type chatResponse struct {
	Choices []struct {
		Message      chatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type errBody struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func (p *Provider) buildRequest(opts llm.CompletionOptions, stream bool) chatRequest {
	model := opts.Model
	if model == "" {
		model = p.Model
	}
	body := chatRequest{
		Model:  model,
		Stream: stream,
		Stop:   opts.StopSequences,
	}
	if opts.MaxTokens > 0 {
		body.MaxTokens = opts.MaxTokens
	}
	if opts.Temperature > 0 {
		t := opts.Temperature
		body.Temperature = &t
	}
	if opts.SystemPrompt != "" {
		body.Messages = append(body.Messages, chatMessage{Role: "system", Content: opts.SystemPrompt})
	}
	for _, m := range opts.Messages {
		body.Messages = append(body.Messages, toOpenAIMessage(m))
	}
	for _, t := range opts.Tools {
		ct := chatTool{Type: "function"}
		ct.Function.Name = t.Name
		ct.Function.Description = t.Description
		ct.Function.Parameters = t.Schema
		body.Tools = append(body.Tools, ct)
	}
	return body
}

func toOpenAIMessage(m llm.Message) chatMessage {
	out := chatMessage{Role: m.Role, Content: m.Content}
	if m.Role == "tool" {
		out.ToolCallID = m.ToolCallID
	}
	for _, tc := range m.ToolCalls {
		argsRaw, _ := json.Marshal(tc.Arguments)
		ct := chatToolCall{ID: tc.ID, Type: "function"}
		ct.Function.Name = tc.Name
		ct.Function.Arguments = string(argsRaw)
		out.ToolCalls = append(out.ToolCalls, ct)
	}
	return out
}

func (p *Provider) doRequest(ctx context.Context, path string, body any) (*http.Response, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.BaseURL+path,
		bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	if p.Org != "" {
		req.Header.Set("OpenAI-Organization", p.Org)
	}
	return p.HTTPClient.Do(req)
}

func (p *Provider) Complete(ctx context.Context, opts llm.CompletionOptions) (*llm.Response, error) {
	body := p.buildRequest(opts, false)
	resp, err := p.doRequest(ctx, "/v1/chat/completions", body)
	if err != nil {
		return nil, fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		var er errBody
		_ = json.Unmarshal(raw, &er)
		return nil, fmt.Errorf("openai %d: %s", resp.StatusCode, er.Error.Message)
	}
	var cr chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if len(cr.Choices) == 0 {
		return nil, errors.New("empty choices")
	}
	ch := cr.Choices[0]
	out := &llm.Response{
		Content:      ch.Message.Content,
		Model:        cr.Model,
		FinishReason: ch.FinishReason,
		Usage: llm.Usage{
			PromptTokens:     cr.Usage.PromptTokens,
			CompletionTokens: cr.Usage.CompletionTokens,
			TotalTokens:      cr.Usage.TotalTokens,
		},
	}
	for _, tc := range ch.Message.ToolCalls {
		var args map[string]any
		_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)
		out.ToolCalls = append(out.ToolCalls, llm.ToolCall{
			ID: tc.ID, Name: tc.Function.Name, Arguments: args,
		})
	}
	return out, nil
}

func (p *Provider) CompleteStream(ctx context.Context, opts llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	body := p.buildRequest(opts, true)
	resp, err := p.doRequest(ctx, "/v1/chat/completions", body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var er errBody
		_ = json.Unmarshal(raw, &er)
		return nil, fmt.Errorf("openai %d: %s", resp.StatusCode, er.Error.Message)
	}

	out := make(chan llm.StreamChunk, 16)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if errors.Is(err, io.EOF) {
					out <- llm.StreamChunk{Done: true}
				}
				return
			}
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			payload := strings.TrimPrefix(line, "data: ")
			if payload == "[DONE]" {
				out <- llm.StreamChunk{Done: true}
				return
			}
			var ev struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(payload), &ev); err != nil {
				continue
			}
			if len(ev.Choices) > 0 && ev.Choices[0].Delta.Content != "" {
				out <- llm.StreamChunk{Delta: ev.Choices[0].Delta.Content}
			}
		}
	}()
	return out, nil
}

// --- Embeddings ---

type EmbedderConfig struct {
	APIKey     string
	BaseURL    string
	Model      string // "text-embedding-3-small" (1536), "text-embedding-3-large" (3072)
	HTTPClient *http.Client
}

func NewEmbedder(cfg EmbedderConfig) *Embedder {
	if cfg.BaseURL == "" {
		cfg.BaseURL = defaultBaseURL
	}
	if cfg.Model == "" {
		cfg.Model = defaultEmbedModel
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 1 * time.Minute}
	}
	return &Embedder{cfg: cfg}
}

type Embedder struct {
	cfg EmbedderConfig
}

func (e *Embedder) Dimensions() int {
	// text-embedding-3-small = 1536 dims (default)
	// text-embedding-3-large = 3072 dims (no compatible con tabla observations.embedding vector(1536))
	return 1536
}

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if e.cfg.APIKey == "" {
		return nil, errors.New("openai api key not configured")
	}
	body := embedRequest{Model: e.cfg.Model, Input: []string{text}}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		e.cfg.BaseURL+"/v1/embeddings", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+e.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := e.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openai embed request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("openai %d: %s", resp.StatusCode, string(msg))
	}
	var er embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&er); err != nil {
		return nil, err
	}
	if len(er.Data) == 0 {
		return nil, errors.New("empty embedding response")
	}
	src := er.Data[0].Embedding
	out := make([]float32, len(src))
	for i, f := range src {
		out[i] = float32(f)
	}
	return out, nil
}
