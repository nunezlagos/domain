package domain

import (
	"context"
	"net/http"
	"net/url"
)

type SkillsResource struct{ c *Client }

type ListSkillsParams struct {
	Type  string
	Tag   string
	Limit int
}

func (p ListSkillsParams) values() url.Values {
	v := url.Values{}
	if p.Type != "" {
		v.Set("type", p.Type)
	}
	if p.Tag != "" {
		v.Set("tag", p.Tag)
	}
	if p.Limit > 0 {
		v.Set("limit", itoa(p.Limit))
	}
	return v
}

func (r *SkillsResource) List(ctx context.Context, params ListSkillsParams) ([]Skill, error) {
	var out []Skill
	_, err := r.c.do(ctx, http.MethodGet, "/skills", params.values(), nil, &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *SkillsResource) Get(ctx context.Context, idOrSlug string) (*Skill, error) {
	var out Skill
	_, err := r.c.do(ctx, http.MethodGet, "/skills/"+idOrSlug, nil, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// Create acepta un map abierto porque el shape de spec varía por skill type.
func (r *SkillsResource) Create(ctx context.Context, in map[string]any) (*Skill, error) {
	var out Skill
	_, err := r.c.do(ctx, http.MethodPost, "/skills", nil, in, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *SkillsResource) Update(ctx context.Context, idOrSlug string, in map[string]any) (*Skill, error) {
	var out Skill
	_, err := r.c.do(ctx, http.MethodPatch, "/skills/"+idOrSlug, nil, in, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *SkillsResource) Delete(ctx context.Context, idOrSlug string) error {
	_, err := r.c.do(ctx, http.MethodDelete, "/skills/"+idOrSlug, nil, nil, nil)
	return err
}
