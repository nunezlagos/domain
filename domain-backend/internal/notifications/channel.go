// Package notifications — issue-20.1 channel abstraction + delivery log.
//
// Cada canal implementa Channel. El Registry permite resolver por slug
// y el Sender orquesta envío + persistencia del delivery log.
//
// Canales actuales:
//   - log_only   — solo log slog (no envía nada)
//   - slack      — webhook Slack (issue-20.3)
//   - email_smtp — SMTP genérico (issue-20.2 ya existe en internal/mail/smtp)
package notifications

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"text/template"

	"github.com/google/uuid"
)

// Errores.
var (
	ErrChannelNotFound  = errors.New("channel_not_found")
	ErrInvalidMessage   = errors.New("invalid_message")
	ErrTemplateNotFound = errors.New("template_not_found")
)

// Message es el payload genérico a enviar por cualquier canal.
type Message struct {
	OrganizationID *uuid.UUID
	TemplateSlug   string
	Recipient      string // canal-específico: email, webhook URL, channel id, etc.
	Subject        string
	Body           string
	Metadata       map[string]any
}

// DeliveryResult retorna info del envío para persistencia.
type DeliveryResult struct {
	Status        string // sent | failed | retrying | dead
	ResponseCode  int
	ErrorMessage  string
	LatencyMS     int
}

// Channel interface implementada por cada canal concreto.
type Channel interface {
	Slug() string
	Send(ctx context.Context, msg Message) (DeliveryResult, error)
}

// Registry thread-safe de canales registrados.
type Registry struct {
	mu       sync.RWMutex
	channels map[string]Channel
}

// NewRegistry constructor.
func NewRegistry() *Registry {
	return &Registry{channels: map[string]Channel{}}
}

// Register agrega un canal por su slug. Sobrescribe si existe (último gana).
func (r *Registry) Register(ch Channel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channels[ch.Slug()] = ch
}

// Get retorna el canal por slug o ErrChannelNotFound.
func (r *Registry) Get(slug string) (Channel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ch, ok := r.channels[slug]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrChannelNotFound, slug)
	}
	return ch, nil
}

// Slugs lista los canales registrados (útil para validation + UI).
func (r *Registry) Slugs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.channels))
	for s := range r.channels {
		out = append(out, s)
	}
	return out
}

// Template wrapper sobre text/template con strict missing key check.
type Template struct {
	Slug    string
	Subject string
	Body    string
}

// Render aplica vars al Body (y al Subject si no está vacío).
// Variables faltantes producen error explícito (NO "<no value>").
func (t *Template) Render(vars map[string]any) (string, string, error) {
	body, err := renderStrict("body", t.Body, vars)
	if err != nil {
		return "", "", fmt.Errorf("render body: %w", err)
	}
	var subject string
	if t.Subject != "" {
		s, err := renderStrict("subject", t.Subject, vars)
		if err != nil {
			return "", "", fmt.Errorf("render subject: %w", err)
		}
		subject = s
	}
	return subject, body, nil
}

func renderStrict(name, src string, vars map[string]any) (string, error) {
	tmpl, err := template.New(name).Option("missingkey=error").Parse(src)
	if err != nil {
		return "", err
	}
	var sb stringBuilder
	if err := tmpl.Execute(&sb, vars); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// stringBuilder minimal sin import strings.
type stringBuilder struct{ b []byte }

func (s *stringBuilder) Write(p []byte) (int, error) {
	s.b = append(s.b, p...)
	return len(p), nil
}
func (s *stringBuilder) String() string { return string(s.b) }
