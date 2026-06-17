package domain

import (
	"context"
	"net/http"
)

// OrganizationsResource — single-org (issue-21.5): solo lectura/ajuste de la
// única org y listado de members. El lifecycle multi-org (Create/Delete) se
// removió del backend; aquí también.
type OrganizationsResource struct{ c *Client }

type OrganizationUpdateInput struct {
	Name     *string        `json:"name,omitempty"`
	Settings map[string]any `json:"settings,omitempty"`
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

func (r *OrganizationsResource) ListMembers(ctx context.Context, orgID string) ([]Member, error) {
	var out []Member
	_, err := r.c.do(ctx, http.MethodGet, "/organizations/"+orgID+"/members", nil, nil, &out)
	if err != nil {
		return nil, err
	}
	return out, nil
}
