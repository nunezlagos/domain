package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"

	"nunezlagos/domain/internal/service/openspec"
)

// renderFromDB reconstruye el árbol de un issue desde la DB para comparar
// hashes. reqSlug y fecha quedan vacíos a propósito: no entran en los hashes
// de los archivos trackeados, solo en .openspec.yaml y el path del directorio.
func (h *openspecHandlers) renderFromDB(ctx context.Context, issueID uuid.UUID, slug string) (*openspec.Rendered, error) {
	iss, err := h.issuesR.GetByID(ctx, issueID)
	if err != nil || iss == nil {
		return nil, fmt.Errorf("issue no encontrado en DB")
	}
	change := &openspec.Change{
		IssueID: issueID.String(), IssueSlug: slug, Title: iss.Title,
		Status: iss.Status, Priority: iss.Priority,
	}
	change.Proposal, change.ProposalVersion = h.loadProposalDoc(ctx, issueID)
	change.Design, change.DesignVersion = h.loadDesignDoc(ctx, issueID)
	change.Tasks = h.loadTaskDocs(ctx, issueID)
	scenarios, err := h.loadScenarioDocs(ctx, issueID)
	if err != nil {
		return nil, err
	}
	change.Scenarios = scenarios
	return openspec.Render(change, false, ""), nil
}

type issueRow struct {
	id          uuid.UUID
	slug        string
	title       string
	status      string
	priority    string
	reqSlug     string
	updatedDate string
}

func (h *openspecHandlers) queryIssues(ctx context.Context, projectID uuid.UUID, scope string) ([]issueRow, error) {
	sql := `SELECT i.id, i.slug, i.title, i.status, i.priority, COALESCE(r.slug,''),
	               to_char(i.updated_at, 'YYYY-MM-DD')
	          FROM issues i
	          LEFT JOIN sdd_requirements r ON r.id = i.req_id
	         WHERE i.project_id = $1`
	args := []any{projectID}
	if scope != "all" {
		sql += ` AND i.status = ANY($2::text[])`
		args = append(args, []string{"proposed", "active"})
	}
	sql += ` ORDER BY i.slug`
	rows, err := h.q(ctx).Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []issueRow
	for rows.Next() {
		var r issueRow
		if err := rows.Scan(&r.id, &r.slug, &r.title, &r.status, &r.priority, &r.reqSlug, &r.updatedDate); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (h *openspecHandlers) renderIssue(ctx context.Context, row issueRow) (map[string]any, error) {
	change := &openspec.Change{
		IssueID: row.id.String(), IssueSlug: row.slug, Title: row.title,
		ReqSlug: row.reqSlug, Status: row.status, Priority: row.priority,
	}
	change.Proposal, change.ProposalVersion = h.loadProposalDoc(ctx, row.id)
	change.Design, change.DesignVersion = h.loadDesignDoc(ctx, row.id)
	change.Tasks = h.loadTaskDocs(ctx, row.id)
	scenarios, err := h.loadScenarioDocs(ctx, row.id)
	if err != nil {
		return nil, err
	}
	change.Scenarios = scenarios
	archived := row.status == "implemented" || row.status == "archived"
	rendered := openspec.Render(change, archived, row.updatedDate)
	files := make(map[string]string, len(rendered.Files))
	for name, body := range rendered.Files {
		files[rendered.Dir+name] = body
	}
	return map[string]any{
		"issue_slug": row.slug, "dir": rendered.Dir,
		"status": row.status, "files": files,
	}, nil
}

func (h *openspecHandlers) loadProposalDoc(ctx context.Context, issueID uuid.UUID) (*openspec.ProposalDoc, int) {
	p, err := h.specR.GetLatestProposal(ctx, issueID)
	if err != nil || p == nil {
		return nil, 0
	}
	return &openspec.ProposalDoc{
		Intention: p.Intention, Scope: p.Scope, Approach: p.Approach,
		Risks: ptrStr(p.Risks), TestingNotes: ptrStr(p.TestingNotes),
	}, p.Version
}

func (h *openspecHandlers) loadDesignDoc(ctx context.Context, issueID uuid.UUID) (*openspec.DesignDoc, int) {
	d, err := h.specR.GetLatestDesign(ctx, issueID)
	if err != nil || d == nil {
		return nil, 0
	}
	return &openspec.DesignDoc{
		ArchDecisions: d.ArchDecisions, Alternatives: ptrStr(d.Alternatives),
		DataFlow: ptrStr(d.DataFlow), TDDPlan: ptrStr(d.TDDPlan),
		RisksMitigation: ptrStr(d.RisksMitigation),
	}, d.Version
}

func (h *openspecHandlers) loadTaskDocs(ctx context.Context, issueID uuid.UUID) []openspec.TaskDoc {
	tasks, err := h.tasksR.ListTasks(ctx, issueID)
	if err != nil {
		return nil
	}
	out := make([]openspec.TaskDoc, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, openspec.TaskDoc{
			ID: t.ID.String(), Section: t.Section, Text: t.Description,
			Completed: t.Status == "completed",
		})
	}
	return out
}

func (h *openspecHandlers) loadScenarioDocs(ctx context.Context, issueID uuid.UUID) ([]openspec.ScenarioDoc, error) {
	rows, err := h.q(ctx).Query(ctx,
		`SELECT feature, scenario, given, when_text, then_rows
		   FROM issue_gherkin_scenarios
		  WHERE issue_id = $1 ORDER BY position`, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []openspec.ScenarioDoc
	for rows.Next() {
		var sc openspec.ScenarioDoc
		if err := rows.Scan(&sc.Feature, &sc.Scenario, &sc.Given, &sc.When, &sc.Then); err != nil {
			return nil, err
		}
		out = append(out, sc)
	}
	return out, rows.Err()
}

func (h *openspecHandlers) statusForChange(ctx context.Context, dir string, files map[string]string) map[string]any {
	metaRaw, ok := files[".openspec.yaml"]
	if !ok {
		return map[string]any{"dir": dir, "verdict": "error", "reason": "falta .openspec.yaml"}
	}
	m := openspec.ParseMeta(metaRaw)
	issueID, err := uuid.Parse(m.IssueID)
	if err != nil {
		return map[string]any{"dir": dir, "verdict": "error", "reason": "issue_id inválido en meta"}
	}
	dbRendered, derr := h.renderFromDB(ctx, issueID, m.IssueSlug)
	if derr != nil {
		return map[string]any{"dir": dir, "verdict": "db_missing", "reason": derr.Error()}
	}
	perFile := map[string]string{}
	overall := "clean"
	for _, name := range trackedFiles(m.IssueSlug) {
		v := classifyDrift(m.Hashes[name], openspec.ContentHash(files[name]), dbRendered.Hashes[name])
		perFile[name] = v
		overall = escalate(overall, v)
	}
	return map[string]any{"dir": dir, "issue_slug": m.IssueSlug, "verdict": overall, "files": perFile}
}

func classifyDrift(stored, repoNow, dbNow string) string {
	repoChanged := repoNow != stored
	dbChanged := dbNow != stored
	switch {
	case repoChanged && dbChanged:
		return "conflict"
	case repoChanged:
		return "repo_modified"
	case dbChanged:
		return "db_modified"
	default:
		return "clean"
	}
}

func escalate(overall, v string) string {
	rank := map[string]int{"clean": 0, "repo_modified": 1, "db_modified": 2, "conflict": 3}
	if rank[v] > rank[overall] {
		return v
	}
	return overall
}

func trackedFiles(slug string) []string {
	return []string{"proposal.md", "design.md", "tasks.md", "specs/" + slug + "/spec.md"}
}

func groupFilesByChange(req mcp.CallToolRequest) (map[string]map[string]string, *mcp.CallToolResult) {
	rawFiles, _ := req.GetArguments()["files"].([]any)
	if len(rawFiles) == 0 {
		return nil, mcp.NewToolResultError("files requerido (no vacío)")
	}
	byDir := map[string]map[string]string{}
	for _, raw := range rawFiles {
		mp, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		path, _ := mp["path"].(string)
		content, _ := mp["content"].(string)
		if path == "" {
			continue
		}
		dir := changeDirOf(path)
		if byDir[dir] == nil {
			byDir[dir] = map[string]string{}
		}
		byDir[dir][strings.TrimPrefix(path, dir)] = content
	}
	return byDir, nil
}

func changeDirOf(path string) string {
	if i := strings.Index(path, "/specs/"); i >= 0 {
		return path[:i+1]
	}
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[:i+1]
	}
	return ""
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
