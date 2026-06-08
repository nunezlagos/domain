package domain

import (
	"context"
	"net/http"
)

type AgentsResource struct{ c *Client }

type AgentRunInput struct {
	Input     string         `json:"input"`
	Variables map[string]any `json:"variables,omitempty"`
}

func (r *AgentsResource) List(ctx context.Context) ([]Agent, error) {
	var out []Agent
	_, err := r.c.do(ctx, http.MethodGet, "/agents", nil, nil, &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *AgentsResource) Get(ctx context.Context, idOrSlug string) (*Agent, error) {
	var out Agent
	_, err := r.c.do(ctx, http.MethodGet, "/agents/"+idOrSlug, nil, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *AgentsResource) Create(ctx context.Context, in map[string]any) (*Agent, error) {
	var out Agent
	_, err := r.c.do(ctx, http.MethodPost, "/agents", nil, in, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *AgentsResource) Update(ctx context.Context, idOrSlug string, in map[string]any) (*Agent, error) {
	var out Agent
	_, err := r.c.do(ctx, http.MethodPatch, "/agents/"+idOrSlug, nil, in, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *AgentsResource) Delete(ctx context.Context, idOrSlug string) error {
	_, err := r.c.do(ctx, http.MethodDelete, "/agents/"+idOrSlug, nil, nil, nil)
	return err
}

// Run dispara la ejecución del agente y devuelve el resultado (puede ser
// status "running" si el endpoint es async — el caller debe pollear via Get).
func (r *AgentsResource) Run(ctx context.Context, agentID string, in AgentRunInput) (*RunResult, error) {
	var out RunResult
	_, err := r.c.do(ctx, http.MethodPost, "/agents/"+agentID+"/run", nil, in, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *AgentsResource) RunLogs(ctx context.Context, runID string) ([]map[string]any, error) {
	var out []map[string]any
	_, err := r.c.do(ctx, http.MethodGet, "/agent-runs/"+runID+"/logs", nil, nil, &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}
