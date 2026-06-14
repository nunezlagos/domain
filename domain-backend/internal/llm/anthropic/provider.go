// Package anthropic — Claude provider implementation.
//
// Usa la API REST oficial de Anthropic Messages API
// (https://api.anthropic.com/v1/messages). Sin dep external — solo net/http.
package anthropic

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

	"nunezlagos/domain/internal/llm"
)

const (
	defaultBaseURL    = "https://api.anthropic.com"
	defaultAPIVersion = "2023-06-01"
	defaultModel      = "claude-sonnet-4-5"
)

type Provider struct {
	APIKey     string
	BaseURL    string
	APIVersion string
	HTTPClient *http.Client
	Model      string // default si CompletionOptions.Model está vacío
}

func New(apiKey string) *Provider {
	return &Provider{
		APIKey:     apiKey,
		BaseURL:    defaultBaseURL,
		APIVersion: defaultAPIVersion,
		HTTPClient: &http.Client{Timeout: 5 * time.Minute},
		Model:      defaultModel,
	}
}

func (p *Provider) Name() string { return "anthropic" }

// requestMessage formato del request body.
type requestMessage struct {
	Role    string                `json:"role"`
	Content []requestContentBlock `json:"content"`
}

type requestContentBlock struct {
	Type      string         `json:"type"`              // "text" | "tool_use" | "tool_result"
	Text      string         `json:"text,omitempty"`
	ID        string         `json:"id,omitempty"`
	Name      string         `json:"name,omitempty"`
	Input     map[string]any `json:"input,omitempty"`
	ToolUseID string         `json:"tool_use_id,omitempty"`
	Content   string         `json:"content,omitempty"`
}

type requestBody struct {
	Model       string           `json:"model"`
	System      string           `json:"system,omitempty"`
	Messages    []requestMessage `json:"messages"`
	MaxTokens   int              `json:"max_tokens"`
	Temperature *float64         `json:"temperature,omitempty"`
	Stop        []string         `json:"stop_sequences,omitempty"`
	Tools       []anthropicTool  `json:"tools,omitempty"`
	Stream      bool             `json:"stream,omitempty"`
}

type anthropicTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	InputSchema map[string]any `json:"input_schema"`
}

type responseBody struct {
	ID         string                 `json:"id"`
	Model      string                 `json:"model"`
	StopReason string                 `json:"stop_reason"`
	Content    []responseContentBlock `json:"content"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type responseContentBlock struct {
	Type  string         `json:"type"`
	Text  string         `json:"text,omitempty"`
	ID    string         `json:"id,omitempty"`
	Name  string         `json:"name,omitempty"`
	Input map[string]any `json:"input,omitempty"`
}

type errorResponse struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (p *Provider) buildRequest(opts llm.CompletionOptions, stream bool) requestBody {
	model := opts.Model
	if model == "" {
		model = p.Model
	}
	maxTokens := opts.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}
	body := requestBody{
		Model:     model,
		System:    opts.SystemPrompt,
		MaxTokens: maxTokens,
		Stop:      opts.StopSequences,
		Stream:    stream,
	}
	if opts.Temperature > 0 {
		t := opts.Temperature
		body.Temperature = &t
	}
	for _, t := range opts.Tools {
		body.Tools = append(body.Tools, anthropicTool{
			Name: t.Name, Description: t.Description, InputSchema: t.Schema,
		})
	}
	for _, m := range opts.Messages {
		body.Messages = append(body.Messages, toAnthropicMessage(m))
	}
	return body
}

func toAnthropicMessage(m llm.Message) requestMessage {
	rm := requestMessage{Role: m.Role}
	if m.Role == "tool" {
		// Tool result se manda como user message con content type tool_result
		rm.Role = "user"
		rm.Content = []requestContentBlock{{
			Type: "tool_result", ToolUseID: m.ToolCallID, Content: m.Content,
		}}
		return rm
	}
	if len(m.ToolCalls) > 0 {
		// Assistant que devolvió tool_use blocks
		for _, tc := range m.ToolCalls {
			rm.Content = append(rm.Content, requestContentBlock{
				Type: "tool_use", ID: tc.ID, Name: tc.Name, Input: tc.Arguments,
			})
		}
		if m.Content != "" {
			rm.Content = append([]requestContentBlock{{Type: "text", Text: m.Content}}, rm.Content...)
		}
		return rm
	}
	rm.Content = []requestContentBlock{{Type: "text", Text: m.Content}}
	return rm
}

func (p *Provider) doRequest(ctx context.Context, body any) (*http.Response, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.BaseURL+"/v1/messages", bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.APIKey)
	req.Header.Set("anthropic-version", p.APIVersion)
	return p.HTTPClient.Do(req)
}

// Complete envía request síncrono y retorna Response completa.
func (p *Provider) Complete(ctx context.Context, opts llm.CompletionOptions) (*llm.Response, error) {
	body := p.buildRequest(opts, false)
	resp, err := p.doRequest(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		var er errorResponse
		_ = json.Unmarshal(raw, &er)
		return nil, fmt.Errorf("anthropic %d: %s", resp.StatusCode, er.Error.Message)
	}

	var rb responseBody
	if err := json.NewDecoder(resp.Body).Decode(&rb); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	out := &llm.Response{
		Model:        rb.Model,
		FinishReason: normalizeStopReason(rb.StopReason),
		Usage: llm.Usage{
			PromptTokens:     rb.Usage.InputTokens,
			CompletionTokens: rb.Usage.OutputTokens,
			TotalTokens:      rb.Usage.InputTokens + rb.Usage.OutputTokens,
		},
	}
	var sb strings.Builder
	for _, c := range rb.Content {
		switch c.Type {
		case "text":
			sb.WriteString(c.Text)
		case "tool_use":
			out.ToolCalls = append(out.ToolCalls, llm.ToolCall{
				ID: c.ID, Name: c.Name, Arguments: c.Input,
			})
		}
	}
	out.Content = sb.String()
	return out, nil
}

// CompleteStream consume server-sent events de la API y emite chunks.
func (p *Provider) CompleteStream(ctx context.Context, opts llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	body := p.buildRequest(opts, true)
	resp, err := p.doRequest(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("anthropic stream request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var er errorResponse
		_ = json.Unmarshal(raw, &er)
		return nil, fmt.Errorf("anthropic %d: %s", resp.StatusCode, er.Error.Message)
	}

	out := make(chan llm.StreamChunk, 16)
	go func() {
		defer close(out)
		defer resp.Body.Close()

		var usage llm.Usage
		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if errors.Is(err, io.EOF) {
					out <- llm.StreamChunk{Done: true, Usage: &usage}
				}
				return
			}
			line = strings.TrimSpace(line)
			if line == "" || !strings.HasPrefix(line, "data: ") {
				continue
			}
			payload := strings.TrimPrefix(line, "data: ")
			if payload == "[DONE]" {
				out <- llm.StreamChunk{Done: true, Usage: &usage}
				return
			}
			var ev struct {
				Type  string `json:"type"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
				Usage struct {
					InputTokens  int `json:"input_tokens"`
					OutputTokens int `json:"output_tokens"`
				} `json:"usage"`
			}
			if err := json.Unmarshal([]byte(payload), &ev); err != nil {
				continue
			}
			switch ev.Type {
			case "content_block_delta":
				if ev.Delta.Text != "" {
					out <- llm.StreamChunk{Delta: ev.Delta.Text}
				}
			case "message_delta":
				if ev.Usage.OutputTokens > 0 {
					usage.CompletionTokens = ev.Usage.OutputTokens
					usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
				}
			case "message_start":
				if ev.Usage.InputTokens > 0 {
					usage.PromptTokens = ev.Usage.InputTokens
				}
			}
		}
	}()
	return out, nil
}

func normalizeStopReason(r string) string {
	switch r {
	case "end_turn", "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_use"
	default:
		return r
	}
}

// --- Embeddings via Voyage AI (Anthropic recomienda voyage-3 para embeddings).
// Si VoyageAPIKey vacío, retornar error en Embed.

type EmbedderConfig struct {
	APIKey  string
	BaseURL string
	Model   string // "voyage-3", "voyage-3-large", etc.
	HTTPClient *http.Client
}

func NewEmbedder(cfg EmbedderConfig) *VoyageEmbedder {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.voyageai.com"
	}
	if cfg.Model == "" {
		cfg.Model = "voyage-3"
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 1 * time.Minute}
	}
	return &VoyageEmbedder{cfg: cfg}
}

type VoyageEmbedder struct {
	cfg EmbedderConfig
}

func (v *VoyageEmbedder) Dimensions() int {
	// voyage-3 retorna 1024; padding/truncate al estándar Domain (1536) en service.
	// Para mantener compat con observations.embedding vector(1536) hacemos
	// padding right-zero (no ideal pero unblocking).
	return 1536
}

type voyageRequest struct {
	Model     string   `json:"model"`
	Input     []string `json:"input"`
	InputType string   `json:"input_type,omitempty"`
}

type voyageResponse struct {
	Data []struct {
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

func (v *VoyageEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	all, err := v.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return all[0], nil
}

func (v *VoyageEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if v.cfg.APIKey == "" {
		return nil, errors.New("voyage api key not configured")
	}
	body := voyageRequest{Model: v.cfg.Model, Input: texts}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		v.cfg.BaseURL+"/v1/embeddings", bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+v.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := v.cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("voyage request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("voyage %d: %s", resp.StatusCode, string(msg))
	}
	var vr voyageResponse
	if err := json.NewDecoder(resp.Body).Decode(&vr); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	if len(vr.Data) == 0 {
		return nil, errors.New("empty embedding response")
	}
	out := make([][]float32, len(vr.Data))
	dim := v.Dimensions()
	for i, d := range vr.Data {
		vec := make([]float32, dim)
		for j, f := range d.Embedding {
			if j >= dim {
				break
			}
			vec[j] = float32(f)
		}
		out[i] = vec
	}
	return out, nil
}
