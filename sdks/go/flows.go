package domain

import (
	"context"
	"net/http"
)

type FlowsResource struct{ c *Client }

type FlowRunInput struct {
	Inputs map[string]any `json:"inputs,omitempty"`
}

func (r *FlowsResource) List(ctx context.Context) ([]Flow, error) {
	var out []Flow
	_, err := r.c.do(ctx, http.MethodGet, "/flows", nil, nil, &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *FlowsResource) Get(ctx context.Context, idOrSlug string) (*Flow, error) {
	var out Flow
	_, err := r.c.do(ctx, http.MethodGet, "/flows/"+idOrSlug, nil, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *FlowsResource) Create(ctx context.Context, in map[string]any) (*Flow, error) {
	var out Flow
	_, err := r.c.do(ctx, http.MethodPost, "/flows", nil, in, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *FlowsResource) Update(ctx context.Context, idOrSlug string, in map[string]any) (*Flow, error) {
	var out Flow
	_, err := r.c.do(ctx, http.MethodPatch, "/flows/"+idOrSlug, nil, in, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *FlowsResource) Delete(ctx context.Context, idOrSlug string) error {
	_, err := r.c.do(ctx, http.MethodDelete, "/flows/"+idOrSlug, nil, nil, nil)
	return err
}

func (r *FlowsResource) Run(ctx context.Context, flowID string, in FlowRunInput) (*RunResult, error) {
	var out RunResult
	_, err := r.c.do(ctx, http.MethodPost, "/flows/"+flowID+"/run", nil, in, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
