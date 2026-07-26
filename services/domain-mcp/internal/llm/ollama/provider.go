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
	"os"
	"strings"
	"time"

	"nunezlagos/domain/internal/llm"
)

const (
	defaultBaseURL = "http://localhost:11434"
	defaultModel   = "llama3.1"

	defaultTimeout = 120 * time.Second
)

type Provider struct {
	BaseURL    string
	HTTPClient *http.Client
	Model      string


	AutoPull bool
}

// New construye el provider. Config por env:
//   - DOMAIN_OLLAMA_URL (o DOMAIN_OLLAMA_HOST legacy): base URL
//   - DOMAIN_OLLAMA_AUTO_PULL=true: pull automático de modelos faltantes
func New() *Provider {
	base := defaultBaseURL
	if u := os.Getenv("DOMAIN_OLLAMA_URL"); u != "" {
		base = u
	} else if u := os.Getenv("DOMAIN_OLLAMA_HOST"); u != "" {
		base = u
	}
	return &Provider{
		BaseURL:    base,
		HTTPClient: &http.Client{Timeout: defaultTimeout},
		Model:      defaultModel,
		AutoPull:   os.Getenv("DOMAIN_OLLAMA_AUTO_PULL") == "true",
	}
}

// isModelMissing detecta el error de modelo no descargado.
func isModelMissing(status int, body string) bool {
	return status == http.StatusNotFound && strings.Contains(body, "model")
}

// pullModel descarga el modelo (blocking, stream=false).
func (p *Provider) pullModel(ctx context.Context, model string) error {
	raw, _ := json.Marshal(map[string]any{"name": model, "stream": false})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.BaseURL+"/api/pull", bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("ollama pull: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ollama pull %d: %s", resp.StatusCode, string(msg))
	}
	return nil
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
	return p.complete(ctx, opts, true)
}

func (p *Provider) complete(ctx context.Context, opts llm.CompletionOptions, allowPull bool) (*llm.Response, error) {
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

		if p.AutoPull && allowPull && isModelMissing(resp.StatusCode, string(msg)) {
			if perr := p.pullModel(ctx, body.Model); perr != nil {
				return nil, fmt.Errorf("ollama auto-pull %q: %w", body.Model, perr)
			}
			return p.complete(ctx, opts, false)
		}
		return nil, fmt.Errorf("ollama %d: %s", resp.StatusCode, string(msg))
	}
	var cr chatResp
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, fmt.Errorf("ollama: invalid response: %w", err)
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

// dimsPorModelo: dimensión de salida de cada modelo de embeddings conocido.
//
// DOMAINSERV-157: acá había un `return 1536` fijo, y el esquema pgvector es
// vector(1024) desde la migración 000275 (bge-m3). Como embed() rellenaba el
// vector hasta Dimensions(), el probe de arranque medía SIEMPRE 1536 sin
// importar qué devolviera el modelo: validateDim no encontraba coincidencia y
// degradaba a noop en CADA arranque, dejando la búsqueda semántica apagada en
// producción sin que nada fallara a la vista.
var dimsPorModelo = map[string]int{
	"bge-m3":            1024,
	"nomic-embed-text":  768,
	"mxbai-embed-large": 1024,
	"all-minilm":        384,
}

// dimDesconocida se usa cuando el modelo no está en el mapa. Es deliberadamente
// la dimensión del esquema: un modelo nuevo no debe degradar el embedder por no
// estar en una lista, y si de verdad produce otra cosa el probe lo detecta
// midiendo el vector real —que es justamente lo que el bug impedía—.
const dimDesconocida = 1024

func (e *Embedder) Dimensions() int {
	if d, ok := dimsPorModelo[e.Model]; ok {
		return d
	}
	return dimDesconocida
}

type embedReq struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embedResp struct {
	Embeddings [][]float64 `json:"embeddings"`
}

func (e *Embedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return e.embed(ctx, text)
}

func (e *Embedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v, err := e.embed(ctx, t)
		if err != nil {
			return nil, fmt.Errorf("ollama embed batch %d: %w", i, err)
		}
		out[i] = v
	}
	return out, nil
}

// embed usa /api/embed y NO /api/embeddings: el legacy no trunca y devuelve 500
// "the input length exceeds the context length" en cuanto el texto pasa el num_ctx
// efectivo del modelo (medido en 2048 tokens con bge-m3, ~6000 chars en español).
// Ese error dejaba las observaciones largas con embedding NULL de forma permanente.
func (e *Embedder) embed(ctx context.Context, text string) ([]float32, error) {
	body := embedReq{Model: e.Model, Input: text}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		e.BaseURL+"/api/embed", bytes.NewReader(raw))
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
	if len(er.Embeddings) == 0 || len(er.Embeddings[0]) == 0 {
		return nil, errors.New("empty embedding")
	}
	// DOMAINSERV-157: el largo sale de lo que devolvió el provider, NO de
	// Dimensions(). Antes se rellenaba/truncaba hasta la dimensión declarada, y
	// eso enmascaraba cualquier desalineo: el probe de validateDim medía la
	// constante en vez del modelo, así que un modelo con otra dimensión pasaba
	// desapercibido y uno correcto podía degradarse por una constante mal puesta.
	vec := er.Embeddings[0]
	out := make([]float32, len(vec))
	for i, f := range vec {
		out[i] = float32(f)
	}
	return out, nil
}
