// Package google — Gemini provider (generativelanguage.googleapis.com).
// issue-06.2: Complete + CompleteStream (SSE) con usage metadata.
package google

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
	defaultBaseURL = "https://generativelanguage.googleapis.com"
	defaultModel   = "gemini-2.0-flash"
)

type Provider struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	Model      string
}

func New(apiKey string) *Provider {
	return &Provider{
		APIKey:     apiKey,
		BaseURL:    defaultBaseURL,
		HTTPClient: &http.Client{Timeout: 120 * time.Second},
		Model:      defaultModel,
	}
}

func (p *Provider) Name() string { return "google" }

type part struct {
	Text string `json:"text"`
}

type content struct {
	Role  string `json:"role,omitempty"`
	Parts []part `json:"parts"`
}

type generationConfig struct {
	Temperature     float64  `json:"temperature,omitempty"`
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type genReq struct {
	SystemInstruction *content         `json:"system_instruction,omitempty"`
	Contents          []content        `json:"contents"`
	GenerationConfig  *generationConfig `json:"generationConfig,omitempty"`
}

type usageMetadata struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type genResp struct {
	Candidates []struct {
		Content      content `json:"content"`
		FinishReason string  `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata usageMetadata `json:"usageMetadata"`
	Error         *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func mapRole(role string) string {
	if role == "assistant" {
		return "model"
	}
	return "user"
}

func (p *Provider) buildRequest(opts llm.CompletionOptions) ([]byte, string) {
	model := opts.Model
	if model == "" {
		model = p.Model
	}
	req := genReq{}
	if opts.SystemPrompt != "" {
		req.SystemInstruction = &content{Parts: []part{{Text: opts.SystemPrompt}}}
	}
	for _, m := range opts.Messages {
		req.Contents = append(req.Contents, content{
			Role: mapRole(m.Role), Parts: []part{{Text: m.Content}},
		})
	}
	if opts.Temperature > 0 || opts.MaxTokens > 0 || len(opts.StopSequences) > 0 {
		req.GenerationConfig = &generationConfig{
			Temperature:     opts.Temperature,
			MaxOutputTokens: opts.MaxTokens,
			StopSequences:   opts.StopSequences,
		}
	}
	raw, _ := json.Marshal(req)
	return raw, model
}

func mapFinish(reason string) string {
	switch reason {
	case "MAX_TOKENS":
		return "length"
	default:
		return "stop"
	}
}

func (p *Provider) Complete(ctx context.Context, opts llm.CompletionOptions) (*llm.Response, error) {
	raw, model := p.buildRequest(opts)
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", p.BaseURL, model, p.APIKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		var gr genResp
		if json.Unmarshal(body, &gr) == nil && gr.Error != nil {
			return nil, fmt.Errorf("google %d %s: %s", resp.StatusCode, gr.Error.Status, gr.Error.Message)
		}
		return nil, fmt.Errorf("google %d: %s", resp.StatusCode, string(body))
	}
	var gr genResp
	if err := json.Unmarshal(body, &gr); err != nil {
		return nil, fmt.Errorf("google: invalid response: %w", err)
	}
	if len(gr.Candidates) == 0 {
		return nil, errors.New("google: empty candidates")
	}
	var sb strings.Builder
	for _, pt := range gr.Candidates[0].Content.Parts {
		sb.WriteString(pt.Text)
	}
	return &llm.Response{
		Content:      sb.String(),
		Model:        model,
		FinishReason: mapFinish(gr.Candidates[0].FinishReason),
		Usage: llm.Usage{
			PromptTokens:     gr.UsageMetadata.PromptTokenCount,
			CompletionTokens: gr.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      gr.UsageMetadata.TotalTokenCount,
		},
	}, nil
}

func (p *Provider) CompleteStream(ctx context.Context, opts llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	raw, model := p.buildRequest(opts)
	url := fmt.Sprintf("%s/v1beta/models/%s:streamGenerateContent?alt=sse&key=%s", p.BaseURL, model, p.APIKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google stream: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("google %d: %s", resp.StatusCode, string(body))
	}

	out := make(chan llm.StreamChunk, 16)
	go func() {
		defer close(out)
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		var usage llm.Usage
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			payload := strings.TrimPrefix(line, "data: ")
			var ev genResp
			if err := json.Unmarshal([]byte(payload), &ev); err != nil {
				continue
			}
			if ev.UsageMetadata.TotalTokenCount > 0 {
				usage = llm.Usage{
					PromptTokens:     ev.UsageMetadata.PromptTokenCount,
					CompletionTokens: ev.UsageMetadata.CandidatesTokenCount,
					TotalTokens:      ev.UsageMetadata.TotalTokenCount,
				}
			}
			for _, c := range ev.Candidates {
				for _, pt := range c.Content.Parts {
					if pt.Text != "" {
						out <- llm.StreamChunk{Delta: pt.Text}
					}
				}
			}
		}
		out <- llm.StreamChunk{Done: true, Usage: &usage}
	}()
	return out, nil
}
