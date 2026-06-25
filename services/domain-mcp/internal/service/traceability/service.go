// Package traceability — issue-04.5 forward/backward traceability + dashboards.
package traceability

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Forward trace: REQ → HU → Proposal/Design → Tasks → Code
type RequirementTrace struct {
	Req      RequirementNode `json:"req"`
	Children []HUTraceNode   `json:"children"`
}

type RequirementNode struct {
	ID        uuid.UUID `json:"id"`
	Slug      string    `json:"slug"`
	Title     string    `json:"title"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type HUTraceNode struct {
	HU           UserStorySummary `json:"hu"`
	Proposal     *ProposalSummary `json:"proposal,omitempty"`
	Design       *DesignSummary   `json:"design,omitempty"`
	TaskProgress *TaskProgress    `json:"task_progress,omitempty"`
	CodeRefs     []CodeRefSummary `json:"code_refs,omitempty"`
}

type UserStorySummary struct {
	ID     uuid.UUID `json:"id"`
	Slug   string    `json:"slug"`
	Title  string    `json:"title"`
	Status string    `json:"status"`
}

type ProposalSummary struct {
	Version int    `json:"version"`
	Status  string `json:"status"`
}

type DesignSummary struct {
	Version int    `json:"version"`
	Status  string `json:"status"`
}

type TaskProgress struct {
	Total     int     `json:"total"`
	Completed int     `json:"completed"`
	Pct       float64 `json:"pct"`
}

type CodeRefSummary struct {
	ID       uuid.UUID `json:"id"`
	FilePath string    `json:"file_path"`
	Repo     string    `json:"repo"`
	Branch   *string   `json:"branch,omitempty"`
}

// Backward trace: Code → HU → REQ
type CodeTrace struct {
	File string            `json:"file"`
	HU   *UserStorySummary `json:"hu,omitempty"`
	REQ  *RequirementNode  `json:"req,omitempty"`
}

// Dashboard
type CoverageDashboard struct {
	TotalHUs              int     `json:"total_hus"`
	HUsWithProposal       int     `json:"hus_with_proposal"`
	HUsWithDesign         int     `json:"hus_with_design"`
	HUsWithCompletedTasks int     `json:"hus_with_completed_tasks"`
	HUsWithCodeRefs       int     `json:"hus_with_code_refs"`
	ProposalPct           float64 `json:"proposal_pct"`
	DesignPct             float64 `json:"design_pct"`
	CompletedPct          float64 `json:"completed_pct"`
}

// Progress by REQ
type REQProgressRow struct {
	ReqSlug        string  `json:"req_slug"`
	ReqTitle       string  `json:"req_title"`
	TotalHUs       int     `json:"total_hus"`
	CompletedHUs   int     `json:"completed_hus"`
	TotalTasks     int     `json:"total_tasks"`
	CompletedTasks int     `json:"completed_tasks"`
	TaskPct        float64 `json:"task_pct"`
}

// Cross-reference: HU without proposal
type HUGap struct {
	ID      uuid.UUID `json:"id"`
	Slug    string    `json:"slug"`
	Title   string    `json:"title"`
	ReqSlug string    `json:"req_slug,omitempty"`
}

// Consolidated matrix
type ConsolidatedRow struct {
	ReqSlug         string  `json:"req_slug"`
	ReqTitle        string  `json:"req_title"`
	TotalHUs        int     `json:"total_hus"`
	HUsWithProposal int     `json:"hus_with_proposal"`
	HUsWithDesign   int     `json:"hus_with_design"`
	CompletedHUs    int     `json:"completed_hus"`
	TotalTasks      int     `json:"total_tasks"`
	CompletedTasks  int     `json:"completed_tasks"`
	TaskPct         float64 `json:"task_pct"`
}

// Service provides read-only traceability queries.
type Service struct {
	Pool *pgxpool.Pool
}

// GetRequirementTrace returns full forward trace for a REQ.
func (s *Service) GetRequirementTrace(ctx context.Context, reqSlug string) (*RequirementTrace, error) {
	var req RequirementNode
	err := s.Pool.QueryRow(ctx,
		`SELECT id, slug, title, status, created_at FROM sdd_requirements WHERE slug = $1`, reqSlug,
	).Scan(&req.ID, &req.Slug, &req.Title, &req.Status, &req.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("req not found: %w", err)
	}

	hus, err := s.getHUTraceNodes(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	return &RequirementTrace{Req: req, Children: hus}, nil
}

func (s *Service) getHUTraceNodes(ctx context.Context, reqID uuid.UUID) ([]HUTraceNode, error) {
	rows, err := s.Pool.Query(ctx,
		`SELECT id, slug, title, status FROM issues WHERE req_id = $1 ORDER BY slug`, reqID)
	if err != nil {
		return nil, fmt.Errorf("query HUs: %w", err)
	}
	defer rows.Close()

	var out []HUTraceNode
	for rows.Next() {
		var n HUTraceNode
		if err := rows.Scan(&n.HU.ID, &n.HU.Slug, &n.HU.Title, &n.HU.Status); err != nil {
			return nil, fmt.Errorf("scan HU: %w", err)
		}


		s.Pool.QueryRow(ctx,
			`SELECT version, status FROM sdd_proposals WHERE issue_id = $1 ORDER BY version DESC LIMIT 1`,
			n.HU.ID,
		).Scan(&n.Proposal)


		s.Pool.QueryRow(ctx,
			`SELECT version, status FROM sdd_designs WHERE issue_id = $1 ORDER BY version DESC LIMIT 1`,
			n.HU.ID,
		).Scan(&n.Design)


		var tp TaskProgress
		err := s.Pool.QueryRow(ctx,
			`SELECT COUNT(*), COUNT(*) FILTER (WHERE status = 'completed'),
			        COALESCE(ROUND(100.0 * COUNT(*) FILTER (WHERE status = 'completed') / GREATEST(COUNT(*), 1), 1), 0)
			 FROM issue_tasks WHERE issue_id = $1`, n.HU.ID,
		).Scan(&tp.Total, &tp.Completed, &tp.Pct)
		if err == nil && tp.Total > 0 {
			n.TaskProgress = &tp
		}


		codeRows, err := s.Pool.Query(ctx,
			`SELECT id, file_path, repo, branch FROM issue_code_references WHERE issue_id = $1 ORDER BY file_path`, n.HU.ID)
		if err == nil {
			for codeRows.Next() {
				var cr CodeRefSummary
				if err := codeRows.Scan(&cr.ID, &cr.FilePath, &cr.Repo, &cr.Branch); err == nil {
					n.CodeRefs = append(n.CodeRefs, cr)
				}
			}
			codeRows.Close()
		}

		out = append(out, n)
	}
	return out, nil
}

// GetCodeTrace returns backward trace from a file path.
func (s *Service) GetCodeTrace(ctx context.Context, filePath string) (*CodeTrace, error) {
	var ct CodeTrace
	ct.File = filePath




	var issueID uuid.UUID
	var hu UserStorySummary
	err := s.Pool.QueryRow(ctx,
		`SELECT cr.issue_id, us.slug, us.title, us.status
		 FROM issue_code_references cr
		 JOIN issues us ON us.id = cr.issue_id
		 WHERE cr.file_path = $1
		 LIMIT 1`, filePath,
	).Scan(&issueID, &hu.Slug, &hu.Title, &hu.Status)
	if err != nil {
		return &ct, nil // no trace but no error
	}
	hu.ID = issueID
	ct.HU = &hu

	var req RequirementNode
	err = s.Pool.QueryRow(ctx,
		`SELECT r.id, r.slug, r.title, r.status, r.created_at
		 FROM sdd_requirements r
		 JOIN issues us ON us.req_id = r.id
		 WHERE us.id = $1`, issueID,
	).Scan(&req.ID, &req.Slug, &req.Title, &req.Status, &req.CreatedAt)
	if err == nil {
		ct.REQ = &req
	}

	return &ct, nil
}

// GetCoverageDashboard returns aggregate coverage metrics.
func (s *Service) GetCoverageDashboard(ctx context.Context) (*CoverageDashboard, error) {
	var d CoverageDashboard
	err := s.Pool.QueryRow(ctx, `
		SELECT
			COUNT(DISTINCT us.id) AS total_hus,
			COUNT(DISTINCT us.id) FILTER (WHERE p.issue_id IS NOT NULL) AS hus_with_proposal,
			COUNT(DISTINCT us.id) FILTER (WHERE d.issue_id IS NOT NULL) AS hus_with_design,
			COUNT(DISTINCT us.id) FILTER (WHERE t.id IS NOT NULL AND t.status = 'completed') AS hus_with_completed_tasks,
			COUNT(DISTINCT us.id) FILTER (WHERE cr.issue_id IS NOT NULL) AS hus_with_code_refs
		FROM issues us
		LEFT JOIN LATERAL (SELECT issue_id FROM sdd_proposals WHERE issue_id = us.id LIMIT 1) p ON true
		LEFT JOIN LATERAL (SELECT issue_id FROM sdd_designs WHERE issue_id = us.id LIMIT 1) d ON true
		LEFT JOIN issue_tasks t ON t.issue_id = us.id
		LEFT JOIN LATERAL (SELECT issue_id FROM issue_code_references WHERE issue_id = us.id LIMIT 1) cr ON true
	`).Scan(&d.TotalHUs, &d.HUsWithProposal, &d.HUsWithDesign, &d.HUsWithCompletedTasks, &d.HUsWithCodeRefs)
	if err != nil {
		return nil, fmt.Errorf("coverage dashboard: %w", err)
	}
	if d.TotalHUs > 0 {
		d.ProposalPct = float64(d.HUsWithProposal) / float64(d.TotalHUs) * 100
		d.DesignPct = float64(d.HUsWithDesign) / float64(d.TotalHUs) * 100
		d.CompletedPct = float64(d.HUsWithCompletedTasks) / float64(d.TotalHUs) * 100
	}
	return &d, nil
}

// GetProgressReport returns task progress grouped by REQ.
func (s *Service) GetProgressReport(ctx context.Context) ([]REQProgressRow, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT r.slug, r.title,
			COUNT(DISTINCT us.id) AS total_hus,
			COUNT(DISTINCT us.id) FILTER (WHERE us.status = 'completed') AS completed_hus,
			COUNT(t.id) AS total_tasks,
			COUNT(t.id) FILTER (WHERE t.status = 'completed') AS completed_tasks,
			CASE WHEN COUNT(t.id) > 0
				THEN ROUND(100.0 * COUNT(t.id) FILTER (WHERE t.status = 'completed') / COUNT(t.id), 1)
				ELSE 0
			END AS task_pct
		FROM sdd_requirements r
		LEFT JOIN issues us ON us.req_id = r.id
		LEFT JOIN issue_tasks t ON t.issue_id = us.id
		WHERE r.status = 'active'
		GROUP BY r.slug, r.title
		ORDER BY task_pct ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("progress report: %w", err)
	}
	defer rows.Close()

	var out []REQProgressRow
	for rows.Next() {
		var row REQProgressRow
		if err := rows.Scan(&row.ReqSlug, &row.ReqTitle, &row.TotalHUs, &row.CompletedHUs, &row.TotalTasks, &row.CompletedTasks, &row.TaskPct); err != nil {
			return nil, fmt.Errorf("scan progress: %w", err)
		}
		out = append(out, row)
	}
	return out, nil
}

// GetHUsWithoutProposals returns HUs with no proposal.
func (s *Service) GetHUsWithoutProposals(ctx context.Context) ([]HUGap, error) {
	return s.gapQuery(ctx, `LEFT JOIN LATERAL (SELECT 1 FROM sdd_proposals WHERE issue_id = us.id LIMIT 1) p ON true WHERE p.column1 IS NULL`)
}

// GetHUsWithoutDesigns returns HUs with no design.
func (s *Service) GetHUsWithoutDesigns(ctx context.Context) ([]HUGap, error) {
	return s.gapQuery(ctx, `LEFT JOIN LATERAL (SELECT 1 FROM sdd_designs WHERE issue_id = us.id LIMIT 1) d ON true WHERE d.column1 IS NULL`)
}

// GetHUsWithIncompleteTasks returns HUs where not all tasks completed.
func (s *Service) GetHUsWithIncompleteTasks(ctx context.Context) ([]HUGap, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT us.id, us.slug, us.title, r.slug
		FROM issues us
		LEFT JOIN sdd_requirements r ON r.id = us.req_id
		WHERE us.id IN (
			SELECT issue_id FROM issue_tasks
			GROUP BY issue_id
			HAVING COUNT(*) FILTER (WHERE status = 'completed') < COUNT(*)
		)
		ORDER BY us.slug
	`)
	if err != nil {
		return nil, fmt.Errorf("incomplete tasks: %w", err)
	}
	defer rows.Close()
	return scanGaps(rows)
}

// GetConsolidatedReport returns matrix of REQ stats.
func (s *Service) GetConsolidatedReport(ctx context.Context) ([]ConsolidatedRow, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT
			r.slug, r.title,
			COUNT(DISTINCT us.id) AS total_hus,
			COUNT(DISTINCT us.id) FILTER (WHERE p.issue_id IS NOT NULL) AS hus_with_proposal,
			COUNT(DISTINCT us.id) FILTER (WHERE d.issue_id IS NOT NULL) AS hus_with_design,
			COUNT(DISTINCT us.id) FILTER (WHERE us.status = 'completed') AS completed_hus,
			COUNT(t.id) AS total_tasks,
			COUNT(t.id) FILTER (WHERE t.status = 'completed') AS completed_tasks,
			CASE WHEN COUNT(t.id) > 0
				THEN ROUND(100.0 * COUNT(t.id) FILTER (WHERE t.status = 'completed') / COUNT(t.id), 1)
				ELSE 0
			END AS task_pct
		FROM sdd_requirements r
		LEFT JOIN issues us ON us.req_id = r.id
		LEFT JOIN LATERAL (SELECT issue_id FROM sdd_proposals WHERE issue_id = us.id LIMIT 1) p ON true
		LEFT JOIN LATERAL (SELECT issue_id FROM sdd_designs WHERE issue_id = us.id LIMIT 1) d ON true
		LEFT JOIN issue_tasks t ON t.issue_id = us.id
		WHERE r.status = 'active'
		GROUP BY r.slug, r.title
		ORDER BY r.slug
	`)
	if err != nil {
		return nil, fmt.Errorf("consolidated report: %w", err)
	}
	defer rows.Close()

	var out []ConsolidatedRow
	for rows.Next() {
		var row ConsolidatedRow
		if err := rows.Scan(&row.ReqSlug, &row.ReqTitle,
			&row.TotalHUs, &row.HUsWithProposal, &row.HUsWithDesign, &row.CompletedHUs,
			&row.TotalTasks, &row.CompletedTasks, &row.TaskPct,
		); err != nil {
			return nil, fmt.Errorf("scan consolidated: %w", err)
		}
		out = append(out, row)
	}
	return out, nil
}

// AddCodeReference creates a code reference link.
func (s *Service) AddCodeReference(ctx context.Context, issueID uuid.UUID, filePath, repo, branch string) (*CodeRefSummary, error) {
	var cr CodeRefSummary
	err := s.Pool.QueryRow(ctx,
		`INSERT INTO issue_code_references (issue_id, file_path, repo, branch)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (issue_id, file_path) DO NOTHING
		 RETURNING id, file_path, repo, branch`,
		issueID, filePath, repo, nullStr(branch),
	).Scan(&cr.ID, &cr.FilePath, &cr.Repo, &cr.Branch)
	if err != nil {
		return nil, fmt.Errorf("add code reference: %w", err)
	}
	return &cr, nil
}

// RemoveCodeReference removes a code reference by ID.
func (s *Service) RemoveCodeReference(ctx context.Context, refID uuid.UUID) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM issue_code_references WHERE id = $1`, refID)
	if err != nil {
		return fmt.Errorf("remove code reference: %w", err)
	}
	return nil
}



func (s *Service) gapQuery(ctx context.Context, joinClause string) ([]HUGap, error) {
	q := `SELECT us.id, us.slug, us.title, COALESCE(r.slug, '') FROM issues us
		  LEFT JOIN sdd_requirements r ON r.id = us.req_id ` + joinClause + ` ORDER BY us.slug`
	rows, err := s.Pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("gap query: %w", err)
	}
	defer rows.Close()
	return scanGaps(rows)
}

func scanGaps(rows pgx.Rows) ([]HUGap, error) {
	var out []HUGap
	for rows.Next() {
		var g HUGap
		if err := rows.Scan(&g.ID, &g.Slug, &g.Title, &g.ReqSlug); err != nil {
			return nil, fmt.Errorf("scan gap: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func nullStr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
