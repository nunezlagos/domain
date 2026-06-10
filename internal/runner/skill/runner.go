// Package skillrunner — issue-05.5 ejecución runtime de los 4 skill types.
//
// Types soportados:
//   - prompt  : sustituye {{var}} en content; resultado = template renderizado
//   - api     : HTTP call con URL/method/headers desde content (JSON config)
//   - code    : NotImplemented — requiere sandbox (issue-11.1)
//   - mcp_tool: NotImplemented — requiere MCP forward (issue-12.4)
//
// Cada ejecución se persiste en agent_run.outputs o como skill_run independiente
// (futuro). Por ahora el caller (agentrunner) recibe el resultado string.
package skillrunner

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"nunezlagos/domain/internal/audit"
	skillsvc "nunezlagos/domain/internal/service/skill"
)

var (
	ErrNotImplemented = errors.New("skill type not implemented yet")
	ErrInvalidConfig  = errors.New("invalid skill content config")
	ErrURLNotAllowed  = errors.New("skill api: URL host not in allowlist")
)

// Runner ejecuta skills. Inyectado en agentrunner.
type Runner struct {
	Audit       audit.Recorder
	HTTPClient  *http.Client       // para skill type=api; default 30s timeout
	AllowedHosts map[string]bool   // allowlist hosts permitidos para api skills (nil = todos)
}

func New() *Runner {
	return &Runner{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Execute corre el skill con args y devuelve el resultado como string.
// La validación de InputSchema es responsabilidad del caller (skill.ValidateInput).
func (r *Runner) Execute(ctx context.Context, sk *skillsvc.Skill, args map[string]any) (string, error) {
	if sk == nil {
		return "", errors.New("skill is nil")
	}
	switch sk.SkillType {
	case skillsvc.TypePrompt:
		return r.executePrompt(sk, args)
	case skillsvc.TypeAPI:
		return r.executeAPI(ctx, sk, args)
	case skillsvc.TypeCode:
		return "", fmt.Errorf("%w: skill type 'code' requires sandbox runner (issue-11.1 pending)", ErrNotImplemented)
	case skillsvc.TypeMCPTool:
		return "", fmt.Errorf("%w: skill type 'mcp_tool' requires MCP forward (issue-12.4 pending)", ErrNotImplemented)
	default:
		return "", fmt.Errorf("unknown skill type: %s", sk.SkillType)
	}
}

// --- prompt ---

func (r *Runner) executePrompt(sk *skillsvc.Skill, args map[string]any) (string, error) {
	out := sk.Content
	for k, v := range args {
		placeholder := "{{" + k + "}}"
		out = strings.ReplaceAll(out, placeholder, fmt.Sprint(v))
	}
	// Detectar placeholders no sustituidos
	if idx := strings.Index(out, "{{"); idx >= 0 {
		if end := strings.Index(out[idx:], "}}"); end >= 0 {
			missing := out[idx+2 : idx+end]
			return "", fmt.Errorf("variable not provided: %s", strings.TrimSpace(missing))
		}
	}
	return out, nil
}

// --- api ---

// apiConfig formato del content para skills tipo api.
//
// Ejemplo de content (JSON serializado):
//   {
//     "url": "https://api.github.com/repos/{{owner}}/{{repo}}",
//     "method": "GET",
//     "headers": {"Authorization": "Bearer {{token}}"},
//     "body_template": "..."  // opcional, JSON-stringified
//   }
type apiConfig struct {
	URL          string            `json:"url"`
	Method       string            `json:"method"`
	Headers      map[string]string `json:"headers"`
	BodyTemplate string            `json:"body_template,omitempty"`
}

func (r *Runner) executeAPI(ctx context.Context, sk *skillsvc.Skill, args map[string]any) (string, error) {
	// Renderizar el content con variables primero (los {{vars}} pueden estar en URL/headers/body)
	rendered, err := r.executePrompt(sk, args)
	if err != nil {
		return "", fmt.Errorf("render api template: %w", err)
	}
	var cfg apiConfig
	if err := json.Unmarshal([]byte(rendered), &cfg); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}
	if cfg.URL == "" {
		return "", fmt.Errorf("%w: url required", ErrInvalidConfig)
	}
	if cfg.Method == "" {
		cfg.Method = http.MethodGet
	}
	parsed, err := url.Parse(cfg.URL)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	if r.AllowedHosts != nil && !r.AllowedHosts[parsed.Host] {
		return "", fmt.Errorf("%w: %s", ErrURLNotAllowed, parsed.Host)
	}
	// Rechazar schemes no http/https
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("only http/https schemes allowed (got %s)", parsed.Scheme)
	}

	var body io.Reader
	if cfg.BodyTemplate != "" {
		body = bytes.NewReader([]byte(cfg.BodyTemplate))
	}
	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(cfg.Method), cfg.URL, body)
	if err != nil {
		return "", fmt.Errorf("new request: %w", err)
	}
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}
	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("api skill call: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // cap 1MB
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}
	out := map[string]any{
		"status_code": resp.StatusCode,
		"body":        string(raw),
		"headers":     flattenHeaders(resp.Header),
	}
	rawOut, _ := json.Marshal(out)
	return string(rawOut), nil
}

func flattenHeaders(h http.Header) map[string]string {
	out := map[string]string{}
	for k, v := range h {
		if len(v) > 0 {
			out[k] = v[0]
		}
	}
	return out
}
