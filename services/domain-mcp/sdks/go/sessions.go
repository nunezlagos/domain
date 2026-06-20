package domain

import (
	"context"
	"net/http"
	"net/url"
)

type SessionsResource struct{ c *Client }

type SessionStartInput struct {
	Title       string   `json:"title"`
	ProjectSlug string   `json:"project_slug,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type SessionEndInput struct {
	Summary string `json:"summary,omitempty"`
}

func (r *SessionsResource) Start(ctx context.Context, in SessionStartInput) (*Session, error) {
	var out Session
	_, err := r.c.do(ctx, http.MethodPost, "/sessions", nil, in, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *SessionsResource) End(ctx context.Context, sessionID string, in SessionEndInput) (*Session, error) {
	var out Session
	_, err := r.c.do(ctx, http.MethodPost, "/sessions/"+sessionID+"/end", nil, in, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *SessionsResource) Get(ctx context.Context, sessionID string) (*Session, error) {
	var out Session
	_, err := r.c.do(ctx, http.MethodGet, "/sessions/"+sessionID, nil, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

type ListSessionsParams struct {
	ProjectSlug string
	Limit       int
	Cursor      string
}

func (p ListSessionsParams) values() url.Values {
	v := url.Values{}
	if p.ProjectSlug != "" {
		v.Set("project_slug", p.ProjectSlug)
	}
	if p.Limit > 0 {
		v.Set("limit", itoa(p.Limit))
	}
	if p.Cursor != "" {
		v.Set("cursor", p.Cursor)
	}
	return v
}

func (r *SessionsResource) List(ctx context.Context, params ListSessionsParams) ([]Session, *Pagination, error) {
	var out []Session
	pg, err := r.c.do(ctx, http.MethodGet, "/sessions", params.values(), nil, &out)
	if err != nil {
		return nil, nil, err
	}
	return out, pg, nil
}

// Active devuelve la sesión activa (si existe) para el project_slug dado.
// Si el server devuelve 204/null, esta función retorna (nil, nil).
func (r *SessionsResource) Active(ctx context.Context, projectSlug string) (*Session, error) {
	q := url.Values{}
	if projectSlug != "" {
		q.Set("project_slug", projectSlug)
	}
	var out Session
	_, err := r.c.do(ctx, http.MethodGet, "/sessions/active", q, nil, &out)
	if err != nil {
		if IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	if out.ID == "" {
		return nil, nil
	}
	return &out, nil
}
