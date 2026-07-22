// Package opencode expone un llm.Provider que habla con un sidecar `opencode
// serve` por HTTP. Es el cerebro server-side de prod del epic DOMAINSERV-62:
// el runtime distroless no puede spawnear opencode, así que corre como sidecar.
package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"nunezlagos/domain/internal/llm"
)

// ProviderName es el nombre registrado en el llm.Factory
const ProviderName = "opencode"

// Provider llama la API HTTP de un opencode serve
type Provider struct {
	baseURL string
	httpc   *http.Client
	user    string
	pass    string
}

// New construye el provider apuntando a un opencode serve. httpc nil usa un
// cliente con timeout por default; user/pass habilitan HTTP basic auth
func New(baseURL string, httpc *http.Client, user, pass string) *Provider {
	if httpc == nil {
		httpc = &http.Client{Timeout: 120 * time.Second}
	}
	return &Provider{baseURL: strings.TrimRight(baseURL, "/"), httpc: httpc, user: user, pass: pass}
}

func (p *Provider) Name() string { return ProviderName }

// Complete abre una sesión y manda el prompt; devuelve el texto del asistente
func (p *Provider) Complete(ctx context.Context, opts llm.CompletionOptions) (*llm.Response, error) {
	var sess struct {
		ID string `json:"id"`
	}
	if err := p.post(ctx, "/session", map[string]any{}, &sess); err != nil {
		return nil, fmt.Errorf("opencode session: %w", err)
	}

	body := map[string]any{"parts": []any{map[string]any{"type": "text", "text": composeUser(opts)}}}
	if opts.SystemPrompt != "" {
		body["system"] = opts.SystemPrompt
	}
	if opts.Model != "" {
		body["model"] = opts.Model
	}
	var msg struct {
		Parts []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"parts"`
	}
	if err := p.post(ctx, "/session/"+sess.ID+"/message", body, &msg); err != nil {
		return nil, fmt.Errorf("opencode message: %w", err)
	}

	var out strings.Builder
	for _, part := range msg.Parts {
		if part.Type == "text" {
			out.WriteString(part.Text)
		}
	}
	return &llm.Response{Content: out.String(), Model: opts.Model, FinishReason: "stop"}, nil
}

// CompleteStream degrada a Complete y emite el resultado en un único chunk
func (p *Provider) CompleteStream(ctx context.Context, opts llm.CompletionOptions) (<-chan llm.StreamChunk, error) {
	resp, err := p.Complete(ctx, opts)
	if err != nil {
		return nil, err
	}
	ch := make(chan llm.StreamChunk, 1)
	ch <- llm.StreamChunk{Delta: resp.Content, Done: true, Usage: &resp.Usage}
	close(ch)
	return ch, nil
}

func (p *Provider) post(ctx context.Context, path string, body, out any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if p.user != "" || p.pass != "" {
		req.SetBasicAuth(p.user, p.pass)
	}
	resp, err := p.httpc.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func composeUser(opts llm.CompletionOptions) string {
	var b strings.Builder
	for _, m := range opts.Messages {
		if c := strings.TrimSpace(m.Content); c != "" {
			b.WriteString(c)
			b.WriteString("\n")
		}
	}
	return strings.TrimSpace(b.String())
}
