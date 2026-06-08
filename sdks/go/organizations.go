package domain

import (
	"context"
	"net/http"
)

type OrganizationsResource struct{ c *Client }

type OrganizationCreateInput struct {
	Name       string `json:"name"`
	Slug       string `json:"slug"`
	OwnerEmail string `json:"owner_email"`
	OwnerName  string `json:"owner_name,omitempty"`
}

type OrganizationUpdateInput struct {
	Name     *string        `json:"name,omitempty"`
	Settings map[string]any `json:"settings,omitempty"`
}

func (r *OrganizationsResource) Create(ctx context.Context, in OrganizationCreateInput) (*Organization, error) {
	var out Organization
	_, err := r.c.do(ctx, http.MethodPost, "/organizations", nil, in, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *OrganizationsResource) Get(ctx context.Context, id string) (*Organization, error) {
	var out Organization
	_, err := r.c.do(ctx, http.MethodGet, "/organizations/"+id, nil, nil, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *OrganizationsResource) Update(ctx context.Context, id string, in OrganizationUpdateInput) (*Organization, error) {
	var out Organization
	_, err := r.c.do(ctx, http.MethodPatch, "/organizations/"+id, nil, in, &out)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func (r *OrganizationsResource) Delete(ctx context.Context, id string) error {
	_, err := r.c.do(ctx, http.MethodDelete, "/organizations/"+id, nil, nil, nil)
	return err
}

func (r *OrganizationsResource) ListMembers(ctx context.Context, orgID string) ([]Member, error) {
	var out []Member
	_, err := r.c.do(ctx, http.MethodGet, "/organizations/"+orgID+"/members", nil, nil, &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}
