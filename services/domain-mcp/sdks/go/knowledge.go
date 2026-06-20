package domain

import (
	"context"
	"net/http"
	"net/url"
)

type KnowledgeResource struct{ c *Client }

type KnowledgeSaveInput struct {
	ProjectSlug string   `json:"project_slug"`
	Title       string   `json:"title"`
	Body        string   `json:"body"`
	Source      string   `json:"source,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

func (r *KnowledgeResource) Save(ctx context.Context, in KnowledgeSaveInput) (*KnowledgeSaveResult, error) {
	var out KnowledgeSaveResult
	_, err := r.c.do(ctx, http.MethodPost, "/knowledge", nil, in, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *KnowledgeResource) Get(ctx context.Context, id string) (*Knowledge, error) {
	var out Knowledge
	_, err := r.c.do(ctx, http.MethodGet, "/knowledge/"+id, nil, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *KnowledgeResource) Delete(ctx context.Context, id string) error {
	_, err := r.c.do(ctx, http.MethodDelete, "/knowledge/"+id, nil, nil, nil)
	return err
}

type KnowledgeSearchParams struct {
	Query string
	Limit int
}

func (p KnowledgeSearchParams) values() url.Values {
	v := url.Values{}
	if p.Query != "" {
		v.Set("q", p.Query)
	}
	if p.Limit > 0 {
		v.Set("limit", itoa(p.Limit))
	}
	return v
}

func (r *KnowledgeResource) Search(ctx context.Context, params KnowledgeSearchParams) ([]Knowledge, error) {
	var out []Knowledge
	_, err := r.c.do(ctx, http.MethodGet, "/knowledge/search", params.values(), nil, &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}
