package domain

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

type ObservationsResource struct{ c *Client }

type ObservationCreateInput struct {
	ProjectSlug     string         `json:"project_slug"`
	Content         string         `json:"content"`
	ObservationType string         `json:"observation_type,omitempty"`
	Tags            []string       `json:"tags,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

type ListObservationsParams struct {
	ProjectSlug string
	ProjectID   string
	Limit       int
	Cursor      string
	Tag         string
}

func (p ListObservationsParams) values() url.Values {
	v := url.Values{}
	if p.ProjectSlug != "" {
		v.Set("project_slug", p.ProjectSlug)
	}
	if p.ProjectID != "" {
		v.Set("project_id", p.ProjectID)
	}
	if p.Limit > 0 {
		v.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.Cursor != "" {
		v.Set("cursor", p.Cursor)
	}
	if p.Tag != "" {
		v.Set("tag", p.Tag)
	}
	return v
}

func (r *ObservationsResource) Create(ctx context.Context, in ObservationCreateInput) (*Observation, error) {
	var out Observation
	_, err := r.c.do(ctx, http.MethodPost, "/observations", nil, in, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *ObservationsResource) Get(ctx context.Context, id string) (*Observation, error) {
	var out Observation
	_, err := r.c.do(ctx, http.MethodGet, "/observations/"+id, nil, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// List devuelve una página de observaciones más la metadata de paginación.
func (r *ObservationsResource) List(ctx context.Context, params ListObservationsParams) ([]Observation, *Pagination, error) {
	var out []Observation
	pg, err := r.c.do(ctx, http.MethodGet, "/observations", params.values(), nil, &out)
	if err != nil {
		return nil, nil, err
	}
	return out, pg, nil
}

// Iter devuelve un iterator que cruza todas las páginas transparentemente.
func (r *ObservationsResource) Iter(ctx context.Context, params ListObservationsParams) *Iterator[Observation] {
	fetch := func(ctx context.Context, cursor string) ([]Observation, string, error) {
		p := params
		p.Cursor = cursor
		items, pg, err := r.List(ctx, p)
		if err != nil {
			return nil, "", err
		}
		var next string
		if pg != nil {
			next = pg.NextCursor
		}
		return items, next, nil
	}
	return NewIterator[Observation](fetch)
}

func (r *ObservationsResource) Delete(ctx context.Context, id string) error {
	_, err := r.c.do(ctx, http.MethodDelete, "/observations/"+id, nil, nil, nil)
	return err
}

type SearchObservationsParams struct {
	Query       string
	ProjectSlug string
	Limit       int
	Tags        []string
}

func (p SearchObservationsParams) values() url.Values {
	v := url.Values{}
	if p.Query != "" {
		v.Set("q", p.Query)
	}
	if p.ProjectSlug != "" {
		v.Set("project_slug", p.ProjectSlug)
	}
	if p.Limit > 0 {
		v.Set("limit", strconv.Itoa(p.Limit))
	}
	for _, t := range p.Tags {
		v.Add("tag", t)
	}
	return v
}

func (r *ObservationsResource) Search(ctx context.Context, params SearchObservationsParams) ([]Observation, error) {
	var out []Observation
	_, err := r.c.do(ctx, http.MethodGet, "/observations/search", params.values(), nil, &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}
