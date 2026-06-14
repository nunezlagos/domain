// Package domain provides a Go SDK for the Domain HTTP API.
//
// Uso típico:
//
//	c, err := domain.New(
//	    domain.WithBaseURL("https://api.domain.sh"),
//	    domain.WithAPIKey("domk_live_..."),
//	)
//	if err != nil { ... }
//	proj, err := c.Projects.Get(ctx, "demo")
package domain

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultBaseURL = "http://localhost:8000"
	defaultTimeout = 30 * time.Second
	userAgent      = "domain-sdk-go/0.1.0"
	apiPrefix      = "/api/v1"
)

// Client es el handle principal. Es seguro para uso concurrente — net/http.Client lo es.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	userAgent  string

	Organizations *OrganizationsResource
	Projects      *ProjectsResource
	Observations  *ObservationsResource
	Sessions      *SessionsResource
	Search        *SearchResource
	Skills        *SkillsResource
	Agents        *AgentsResource
	Flows         *FlowsResource
	Knowledge     *KnowledgeResource
}

// Option configura el Client al construirlo.
type Option func(*Client)

// WithBaseURL fija la URL base del API (sin el sufijo /api/v1).
func WithBaseURL(u string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(u, "/") }
}

// WithAPIKey fija la API key. Si no se setea, se lee de DOMAIN_API_KEY.
func WithAPIKey(k string) Option { return func(c *Client) { c.apiKey = k } }

// WithHTTPClient permite inyectar un http.Client custom (p.ej. para tests
// con httptest, para reusar conexiones, o para wrappers de tracing).
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) {
		if h != nil {
			c.httpClient = h
		}
	}
}

// WithTimeout fija el timeout del http.Client por defecto. Si el caller ya
// pasó WithHTTPClient, esta opción NO toca su client.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		if c.httpClient == nil {
			c.httpClient = &http.Client{Timeout: d}
		}
	}
}

// WithUserAgent sobrescribe el User-Agent por defecto.
func WithUserAgent(ua string) Option {
	return func(c *Client) {
		if ua != "" {
			c.userAgent = ua
		}
	}
}

// New construye un Client aplicando las opciones. Devuelve error si no hay
// API key disponible (ni por opción ni por env DOMAIN_API_KEY).
func New(opts ...Option) (*Client, error) {
	c := &Client{
		baseURL:   strings.TrimRight(envOr("DOMAIN_BASE_URL", defaultBaseURL), "/"),
		apiKey:    os.Getenv("DOMAIN_API_KEY"),
		userAgent: userAgent,
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.httpClient == nil {
		c.httpClient = &http.Client{Timeout: defaultTimeout}
	}
	if c.apiKey == "" {
		return nil, errors.New("domain: api_key required (use WithAPIKey or set DOMAIN_API_KEY)")
	}

	c.Organizations = &OrganizationsResource{c: c}
	c.Projects = &ProjectsResource{c: c}
	c.Observations = &ObservationsResource{c: c}
	c.Sessions = &SessionsResource{c: c}
	c.Search = &SearchResource{c: c}
	c.Skills = &SkillsResource{c: c}
	c.Agents = &AgentsResource{c: c}
	c.Flows = &FlowsResource{c: c}
	c.Knowledge = &KnowledgeResource{c: c}

	return c, nil
}

// BaseURL devuelve la URL base configurada (útil para debugging y tests).
func (c *Client) BaseURL() string { return c.baseURL }

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
