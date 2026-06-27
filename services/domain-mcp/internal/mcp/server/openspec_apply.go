package mcpserver

import (
	"context"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"

	issuesvc "nunezlagos/domain/internal/service/issue"
	"nunezlagos/domain/internal/service/openspec"
)

func (h *openspecHandlers) handleApply(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, errResult := h.requireOrg(); errResult != nil {
		return errResult, nil
	}
	byDir, errResult := groupFilesByChange(req)
	if errResult != nil {
		return errResult, nil
	}
	force, _ := req.GetArguments()["force"].(bool)
	results := make([]map[string]any, 0, len(byDir))
	for dir, files := range byDir {
		results = append(results, h.applyChange(ctx, dir, files, force))
	}
	return toolResultJSON(map[string]any{
		"change_count": len(results),
		"changes":      results,
	})
}

func (h *openspecHandlers) applyChange(ctx context.Context, dir string, files map[string]string, force bool) map[string]any {
	m := openspec.ParseMeta(files[".openspec.yaml"])
	issueID, err := uuid.Parse(m.IssueID)
	if err != nil {
		return map[string]any{"dir": dir, "error": "issue_id inválido o falta .openspec.yaml"}
	}
	iss, err := h.issuesR.GetByID(ctx, issueID)
	if err != nil || iss == nil {
		return map[string]any{"dir": dir, "error": "issue no encontrado en DB"}
	}
	dbRendered, err := h.renderFromDB(ctx, issueID, m.IssueSlug)
	if err != nil {
		return map[string]any{"dir": dir, "error": err.Error()}
	}
	applied, conflicts := h.applyFiles(ctx, applyCtx{
		issueID: issueID, iss: iss, slug: m.IssueSlug,
		meta: m, files: files, db: dbRendered, force: force,
	})
	if st := h.applyStatus(ctx, iss, m.Status); st != "" {
		applied = append(applied, st)
	}
	return map[string]any{"dir": dir, "issue_slug": m.IssueSlug, "applied": applied, "conflicts": conflicts}
}

type applyCtx struct {
	issueID uuid.UUID
	iss     *issuesvc.Issue
	slug    string
	meta    openspec.Meta
	files   map[string]string
	db      *openspec.Rendered
	force   bool
}

func (h *openspecHandlers) applyFiles(ctx context.Context, a applyCtx) ([]string, []string) {
	var applied, conflicts []string
	specName := "specs/" + a.slug + "/spec.md"
	steps := []struct {
		name string
		fn   func() error
	}{
		{"proposal.md", func() error { return h.applyProposal(ctx, a.issueID, a.files["proposal.md"]) }},
		{"design.md", func() error { return h.applyDesign(ctx, a.issueID, a.files["design.md"]) }},
		{specName, func() error { return h.applyScenarios(ctx, a.iss, a.files[specName]) }},
		{"tasks.md", func() error { return h.applyTasks(ctx, a.issueID, a.files["tasks.md"]) }},
	}
	for _, s := range steps {
		switch decideFile(a.meta.Hashes[s.name], openspec.ContentHash(a.files[s.name]), a.db.Hashes[s.name], a.force) {
		case "skip":
		case "conflict":
			conflicts = append(conflicts, s.name)
		case "apply":
			if err := s.fn(); err != nil {
				conflicts = append(conflicts, s.name+": "+err.Error())
			} else {
				applied = append(applied, s.name)
			}
		}
	}
	return applied, conflicts
}

func decideFile(stored, repoNow, dbNow string, force bool) string {
	if repoNow == stored {
		return "skip"
	}
	if dbNow != stored && !force {
		return "conflict"
	}
	return "apply"
}

func (h *openspecHandlers) applyProposal(ctx context.Context, issueID uuid.UUID, md string) error {
	p := openspec.ParseProposal(md)
	_, err := h.specW.CreateProposal(ctx, issueID, p.Intention, p.Scope, p.Approach, p.Risks, p.TestingNotes)
	return err
}

func (h *openspecHandlers) applyDesign(ctx context.Context, issueID uuid.UUID, md string) error {
	d := openspec.ParseDesign(md)
	_, err := h.specW.CreateDesign(ctx, issueID, nil, d.ArchDecisions, d.Alternatives, d.DataFlow, d.TDDPlan, d.RisksMitigation)
	return err
}

func (h *openspecHandlers) applyScenarios(ctx context.Context, iss *issuesvc.Issue, md string) error {
	for _, sc := range iss.Scenarios {
		if err := h.issuesW.RemoveScenario(ctx, sc.ID); err != nil {
			return err
		}
	}
	for _, d := range openspec.ParseScenarios(md) {
		sc := issuesvc.Scenario{
			Feature: d.Feature, Scenario: d.Scenario,
			Given: d.Given, When: d.When, Then: d.Then,
		}
		if _, err := h.issuesW.AddScenario(ctx, iss.Slug, sc); err != nil {
			return err
		}
	}
	return nil
}

func (h *openspecHandlers) applyTasks(ctx context.Context, issueID uuid.UUID, md string) error {
	current, err := h.tasksR.ListTasks(ctx, issueID)
	if err != nil {
		return err
	}
	status := map[string]string{}
	for _, t := range current {
		status[t.ID.String()] = t.Status
	}
	by := h.principalUserID()
	for _, td := range openspec.ParseTasks(md) {
		if td.ID == "" || !td.Completed {
			continue
		}
		if err := h.advanceTaskToCompleted(ctx, td.ID, status[td.ID], by); err != nil {
			return err
		}
	}
	return nil
}

func (h *openspecHandlers) advanceTaskToCompleted(ctx context.Context, idStr, current, by string) error {
	taskID, err := uuid.Parse(idStr)
	if err != nil {
		return nil
	}
	if current == "pending" {
		if _, err := h.tasksW.UpdateTaskStatus(ctx, taskID, "in_progress", by); err != nil {
			return err
		}
		current = "in_progress"
	}
	if current == "in_progress" {
		if _, err := h.tasksW.UpdateTaskStatus(ctx, taskID, "completed", by); err != nil {
			return err
		}
	}
	return nil
}

func (h *openspecHandlers) applyStatus(ctx context.Context, iss *issuesvc.Issue, newStatus string) string {
	if newStatus == "" || newStatus == iss.Status {
		return ""
	}
	if _, err := h.issuesW.Update(ctx, iss.Slug, nil, nil, &newStatus, nil); err != nil {
		return "status: " + err.Error()
	}
	return "status -> " + newStatus
}

func (h *openspecHandlers) principalUserID() string {
	if h.principal == nil {
		return "openspec-sync"
	}
	return h.principal.UserID
}
