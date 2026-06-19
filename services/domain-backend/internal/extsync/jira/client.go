// Package jira — issue-04.9 driver Jira Cloud para sync push/pull.
//
// HTTP client minimalista contra Atlassian REST API v3:
//   - CreateIssue para Epic (REQ) o Story (HU)
//   - GetIssue por key (verify sync state)
//   - AddComment, AddAttachment (futuro)
//   - Webhook receiver: parsea changelog → detect drift en summary/description/AC
//
// Auth: basic auth (email + API token) via env vars o auth_secrets table.
// Otros providers (GitHub Issues, Linear, Asana) implementarían misma
// interface ExternalProviderDriver en packages hermanos.
package jira

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client implementa el driver Jira.
type Client struct {
	BaseURL    string // ej: https://acme.atlassian.net
	Email      string
	APIToken   string
	ProjectKey string
	HTTPClient *http.Client
}

// New construye un Client con timeout razonable.
func New(baseURL, email, apiToken, projectKey string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		Email:      email,
		APIToken:   apiToken,
		ProjectKey: projectKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// IssueType para Jira.
type IssueType string

const (
	TypeEpic  IssueType = "Epic"
	TypeStory IssueType = "Story"
	TypeTask  IssueType = "Task"
	TypeBug   IssueType = "Bug"
)

// CreateIssueRequest input.
type CreateIssueRequest struct {
	Summary     string
	Description string
	Type        IssueType
	ParentKey   string // para Story bajo Epic
	Labels      []string
}

// CreateIssueResponse output.
type CreateIssueResponse struct {
	ID  string `json:"id"`
	Key string `json:"key"` // DIDE-100
	URL string `json:"self"`
}

// Issue es la representación lite para pull/get.
type Issue struct {
	ID          string    `json:"id"`
	Key         string    `json:"key"`
	Summary     string    `json:"summary"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateIssue crea Epic o Story.
func (c *Client) CreateIssue(ctx context.Context, in CreateIssueRequest) (*CreateIssueResponse, error) {
	if c.ProjectKey == "" {
		return nil, errors.New("project_key required")
	}
	if in.Type == "" {
		in.Type = TypeStory
	}

	fields := map[string]any{
		"project":   map[string]string{"key": c.ProjectKey},
		"summary":   in.Summary,
		"issuetype": map[string]string{"name": string(in.Type)},
	}
	if in.Description != "" {
		fields["description"] = adfFromMarkdown(in.Description)
	}
	if in.ParentKey != "" {
		fields["parent"] = map[string]string{"key": in.ParentKey}
	}
	if len(in.Labels) > 0 {
		fields["labels"] = in.Labels
	}

	body, _ := json.Marshal(map[string]any{"fields": fields})
	var out CreateIssueResponse
	if err := c.do(ctx, http.MethodPost, "/rest/api/3/issue", body, &out); err != nil {
		return nil, fmt.Errorf("create issue: %w", err)
	}
	return &out, nil
}

// GetIssue devuelve datos de un issue por key.
func (c *Client) GetIssue(ctx context.Context, key string) (*Issue, error) {
	var raw struct {
		ID     string `json:"id"`
		Key    string `json:"key"`
		Fields struct {
			Summary     string `json:"summary"`
			Description any    `json:"description"`
			Status      struct {
				Name string `json:"name"`
			} `json:"status"`
			Updated string `json:"updated"`
		} `json:"fields"`
	}
	if err := c.do(ctx, http.MethodGet, "/rest/api/3/issue/"+key, nil, &raw); err != nil {
		return nil, fmt.Errorf("get issue: %w", err)
	}
	updated, _ := time.Parse("2006-01-02T15:04:05.000-0700", raw.Fields.Updated)
	return &Issue{
		ID:          raw.ID,
		Key:         raw.Key,
		Summary:     raw.Fields.Summary,
		Description: adfToText(raw.Fields.Description),
		Status:      raw.Fields.Status.Name,
		UpdatedAt:   updated,
	}, nil
}

// UpdateIssue actualiza fields del issue.
func (c *Client) UpdateIssue(ctx context.Context, key string, fields map[string]any) error {
	body, _ := json.Marshal(map[string]any{"fields": fields})
	return c.do(ctx, http.MethodPut, "/rest/api/3/issue/"+key, body, nil)
}

// AddComment agrega un comment al issue.
func (c *Client) AddComment(ctx context.Context, key, body string) error {
	payload, _ := json.Marshal(map[string]any{
		"body": adfFromMarkdown(body),
	})
	return c.do(ctx, http.MethodPost, "/rest/api/3/issue/"+key+"/comment", payload, nil)
}

func (c *Client) do(ctx context.Context, method, path string, body []byte, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	req.SetBasicAuth(c.Email, c.APIToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(rb))
	}
	if out != nil && len(rb) > 0 {
		if err := json.Unmarshal(rb, out); err != nil {
			return fmt.Errorf("decode: %w (body=%s)", err, string(rb))
		}
	}
	return nil
}

// adfFromMarkdown convierte markdown plano a Atlassian Document Format
// minimalista (un solo paragraph block con texto). Para format rich, usar
// un parser markdown→ADF dedicado.
func adfFromMarkdown(md string) map[string]any {
	return map[string]any{
		"type":    "doc",
		"version": 1,
		"content": []any{
			map[string]any{
				"type": "paragraph",
				"content": []any{
					map[string]any{
						"type": "text",
						"text": md,
					},
				},
			},
		},
	}
}

// adfToText extrae texto plano de ADF JSON (best effort).
func adfToText(adf any) string {
	if adf == nil {
		return ""
	}
	m, ok := adf.(map[string]any)
	if !ok {
		return fmt.Sprintf("%v", adf)
	}
	var out strings.Builder
	walkADF(m, &out)
	return strings.TrimSpace(out.String())
}

func walkADF(node map[string]any, out *strings.Builder) {
	if t, ok := node["type"].(string); ok && t == "text" {
		if txt, ok := node["text"].(string); ok {
			out.WriteString(txt)
			out.WriteByte(' ')
		}
	}
	if content, ok := node["content"].([]any); ok {
		for _, c := range content {
			if cm, ok := c.(map[string]any); ok {
				walkADF(cm, out)
			}
		}
	}
}

// VerifyWebhookSignature valida HMAC-SHA256 del payload (issue-10.2 pattern).
// Header `X-Hub-Signature-256` viene como "sha256=<hex>".
func VerifyWebhookSignature(payload []byte, signatureHeader, secret string) bool {
	expected := strings.TrimPrefix(signatureHeader, "sha256=")
	if expected == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	actual := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(actual), []byte(expected))
}

// WebhookEvent es el shape relevante del payload Jira (subset).
type WebhookEvent struct {
	Event string `json:"webhookEvent"` // ej: jira:issue_updated
	Issue struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	} `json:"issue"`
	Changelog struct {
		Items []ChangelogItem `json:"items"`
	} `json:"changelog"`
}

// ChangelogItem un campo que cambió.
type ChangelogItem struct {
	Field      string `json:"field"`
	FromString string `json:"fromString"`
	ToString   string `json:"toString"`
}

// ParseWebhook decodifica payload con validación de signature opcional.
func ParseWebhook(payload []byte, signatureHeader, secret string) (*WebhookEvent, error) {
	if secret != "" && !VerifyWebhookSignature(payload, signatureHeader, secret) {
		return nil, errors.New("webhook signature mismatch")
	}
	var ev WebhookEvent
	if err := json.Unmarshal(payload, &ev); err != nil {
		return nil, fmt.Errorf("parse webhook: %w", err)
	}
	return &ev, nil
}

// EncodeBasicAuth helper para tests/debugging.
func EncodeBasicAuth(email, token string) string {
	return base64.StdEncoding.EncodeToString([]byte(email + ":" + token))
}
