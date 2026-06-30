package issuebuilder

import (
	"context"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/requirement"
)

// RequirementServiceAdapter envuelve internal/service/requirement.Service para
// satisfacer la interface lite issuebuilder.RequirementService. Mismo patron
// que AttachmentServiceAdapter: aisla a issuebuilder del tipo concreto. Wire en
// main.go con:
//
//	issuebuilder.Service{ReqSvc: &issuebuilder.RequirementServiceAdapter{Inner: reqSvc}}
type RequirementServiceAdapter struct {
	Inner *requirement.Service
}

// Create satisface issuebuilder.RequirementService.
func (a *RequirementServiceAdapter) Create(ctx context.Context, slug, title, description, status, priority, parentSlug string, projectID *uuid.UUID) (*MaterializedRequirement, error) {
	r, err := a.Inner.Create(ctx, slug, title, description, status, priority, parentSlug, projectID)
	if err != nil {
		return nil, err
	}
	return &MaterializedRequirement{ID: r.ID, Slug: r.Slug}, nil
}
