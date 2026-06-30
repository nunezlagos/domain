package issuebuilder

import (
	"context"

	"nunezlagos/domain/internal/service/issue"
)

// IssueServiceAdapter envuelve internal/service/issue.Service para satisfacer la
// interface lite issuebuilder.IssueService. El draft no genera escenarios
// Gherkin estructurados (solo el issue.md como description), por eso el adapter
// pasa scenarios vacio a issue.Service.Create. Wire en main.go con:
//
//	issuebuilder.Service{IssueSvc: &issuebuilder.IssueServiceAdapter{Inner: issueSvc}}
type IssueServiceAdapter struct {
	Inner *issue.Service
}

// Create satisface issuebuilder.IssueService.
func (a *IssueServiceAdapter) Create(ctx context.Context, slug, title, description, status, priority, reqSlug string) (*MaterializedIssue, error) {
	hu, err := a.Inner.Create(ctx, slug, title, description, status, priority, reqSlug, nil)
	if err != nil {
		return nil, err
	}
	return &MaterializedIssue{ID: hu.ID, Slug: hu.Slug}, nil
}
