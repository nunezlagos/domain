package domain

import (
	"context"
	"net/http"
)

type ProjectsResource struct{ c *Client }

type ProjectCreateInput struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description,omitempty"`
}

type ProjectUpdateInput struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

func (r *ProjectsResource) Create(ctx context.Context, in ProjectCreateInput) (*Project, error) {
	var out Project
	_, err := r.c.do(ctx, http.MethodPost, "/projects", nil, in, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *ProjectsResource) List(ctx context.Context) ([]Project, error) {
	var out []Project
	_, err := r.c.do(ctx, http.MethodGet, "/projects", nil, nil, &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Get acepta UUID o slug; el servidor resuelve ambos contra /projects/{id_or_slug}.
func (r *ProjectsResource) Get(ctx context.Context, idOrSlug string) (*Project, error) {
	var out Project
	_, err := r.c.do(ctx, http.MethodGet, "/projects/"+idOrSlug, nil, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// GetBySlug usa la ruta dedicada /projects/by-slug/{slug}.
func (r *ProjectsResource) GetBySlug(ctx context.Context, slug string) (*Project, error) {
	var out Project
	_, err := r.c.do(ctx, http.MethodGet, "/projects/by-slug/"+slug, nil, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *ProjectsResource) Update(ctx context.Context, idOrSlug string, in ProjectUpdateInput) (*Project, error) {
	var out Project
	_, err := r.c.do(ctx, http.MethodPatch, "/projects/"+idOrSlug, nil, in, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *ProjectsResource) Delete(ctx context.Context, idOrSlug string) error {
	_, err := r.c.do(ctx, http.MethodDelete, "/projects/"+idOrSlug, nil, nil, nil)
	return err
}
