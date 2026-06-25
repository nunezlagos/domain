// Package traceability — issue-04.5 forward/backward traceability + dashboards.
//
//go:generate go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1 generate
package traceability

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/service/traceability/traceabilitydb"
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

func (s *Service) q() *traceabilitydb.Queries { return traceabilitydb.New(s.Pool) }

// GetRequirementTrace returns full forward trace for a REQ.
func (s *Service) GetRequirementTrace(ctx context.Context, reqSlug string) (*RequirementTrace, error) {
	r, err := s.q().GetRequirementBySlug(ctx, reqSlug)
	if err != nil {
		return nil, fmt.Errorf("req not found: %w", err)
	}
	req := RequirementNode{ID: r.ID, Slug: r.Slug, Title: r.Title, Status: r.Status, CreatedAt: r.CreatedAt}

	hus, err := s.getHUTraceNodes(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	return &RequirementTrace{Req: req, Children: hus}, nil
}

func (s *Service) getHUTraceNodes(ctx context.Context, reqID uuid.UUID) ([]HUTraceNode, error) {
	q := s.q()
	issues, err := q.ListIssuesByReq(ctx, reqID)
	if err != nil {
		return nil, fmt.Errorf("query HUs: %w", err)
	}

	var out []HUTraceNode
	for _, hu := range issues {
		n := HUTraceNode{HU: UserStorySummary{ID: hu.ID, Slug: hu.Slug, Title: hu.Title, Status: hu.Status}}

		if p, err := q.LatestProposal(ctx, n.HU.ID); err == nil {
			n.Proposal = &ProposalSummary{Version: int(p.Version), Status: p.Status}
		}

		if d, err := q.LatestDesign(ctx, n.HU.ID); err == nil {
			n.Design = &DesignSummary{Version: int(d.Version), Status: d.Status}
		}

		if tp, err := q.TaskProgressByIssue(ctx, n.HU.ID); err == nil && tp.Total > 0 {
			n.TaskProgress = &TaskProgress{
				Total:     int(tp.Total),
				Completed: int(tp.Completed),
				Pct:       numericToFloat(tp.Pct),
			}
		}

		if refs, err := q.ListCodeRefsByIssue(ctx, n.HU.ID); err == nil {
			for _, r := range refs {
				n.CodeRefs = append(n.CodeRefs, CodeRefSummary{ID: r.ID, FilePath: r.FilePath, Repo: r.Repo, Branch: r.Branch})
			}
		}

		out = append(out, n)
	}
	return out, nil
}

// GetCodeTrace returns backward trace from a file path.
func (s *Service) GetCodeTrace(ctx context.Context, filePath string) (*CodeTrace, error) {
	q := s.q()
	ct := CodeTrace{File: filePath}

	huRow, err := q.GetCodeTraceHU(ctx, filePath)
	if err != nil {
		return &ct, nil // no trace but no error
	}
	ct.HU = &UserStorySummary{ID: huRow.IssueID, Slug: huRow.Slug, Title: huRow.Title, Status: huRow.Status}

	if r, err := q.GetRequirementForIssue(ctx, huRow.IssueID); err == nil {
		ct.REQ = &RequirementNode{ID: r.ID, Slug: r.Slug, Title: r.Title, Status: r.Status, CreatedAt: r.CreatedAt}
	}

	return &ct, nil
}

// GetCoverageDashboard returns aggregate coverage metrics.
func (s *Service) GetCoverageDashboard(ctx context.Context) (*CoverageDashboard, error) {
	row, err := s.q().GetCoverageDashboard(ctx)
	if err != nil {
		return nil, fmt.Errorf("coverage dashboard: %w", err)
	}
	d := CoverageDashboard{
		TotalHUs:              int(row.TotalHus),
		HUsWithProposal:       int(row.HusWithProposal),
		HUsWithDesign:         int(row.HusWithDesign),
		HUsWithCompletedTasks: int(row.HusWithCompletedTasks),
		HUsWithCodeRefs:       int(row.HusWithCodeRefs),
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
	rows, err := s.q().GetProgressReport(ctx)
	if err != nil {
		return nil, fmt.Errorf("progress report: %w", err)
	}
	out := make([]REQProgressRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, REQProgressRow{
			ReqSlug:        r.Slug,
			ReqTitle:       r.Title,
			TotalHUs:       int(r.TotalHus),
			CompletedHUs:   int(r.CompletedHus),
			TotalTasks:     int(r.TotalTasks),
			CompletedTasks: int(r.CompletedTasks),
			TaskPct:        numericToFloat(r.TaskPct),
		})
	}
	return out, nil
}

// GetHUsWithoutProposals returns HUs with no proposal.
func (s *Service) GetHUsWithoutProposals(ctx context.Context) ([]HUGap, error) {
	rows, err := s.q().GetHUsWithoutProposals(ctx)
	if err != nil {
		return nil, fmt.Errorf("gap query: %w", err)
	}
	out := make([]HUGap, 0, len(rows))
	for _, r := range rows {
		out = append(out, HUGap{ID: r.ID, Slug: r.Slug, Title: r.Title, ReqSlug: r.ReqSlug})
	}
	return out, nil
}

// GetHUsWithoutDesigns returns HUs with no design.
func (s *Service) GetHUsWithoutDesigns(ctx context.Context) ([]HUGap, error) {
	rows, err := s.q().GetHUsWithoutDesigns(ctx)
	if err != nil {
		return nil, fmt.Errorf("gap query: %w", err)
	}
	out := make([]HUGap, 0, len(rows))
	for _, r := range rows {
		out = append(out, HUGap{ID: r.ID, Slug: r.Slug, Title: r.Title, ReqSlug: r.ReqSlug})
	}
	return out, nil
}

// GetHUsWithIncompleteTasks returns HUs where not all tasks completed.
func (s *Service) GetHUsWithIncompleteTasks(ctx context.Context) ([]HUGap, error) {
	rows, err := s.q().GetHUsWithIncompleteTasks(ctx)
	if err != nil {
		return nil, fmt.Errorf("incomplete tasks: %w", err)
	}
	out := make([]HUGap, 0, len(rows))
	for _, r := range rows {
		out = append(out, HUGap{ID: r.ID, Slug: r.Slug, Title: r.Title, ReqSlug: r.ReqSlug})
	}
	return out, nil
}

// GetConsolidatedReport returns matrix of REQ stats.
func (s *Service) GetConsolidatedReport(ctx context.Context) ([]ConsolidatedRow, error) {
	rows, err := s.q().GetConsolidatedReport(ctx)
	if err != nil {
		return nil, fmt.Errorf("consolidated report: %w", err)
	}
	out := make([]ConsolidatedRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, ConsolidatedRow{
			ReqSlug:         r.Slug,
			ReqTitle:        r.Title,
			TotalHUs:        int(r.TotalHus),
			HUsWithProposal: int(r.HusWithProposal),
			HUsWithDesign:   int(r.HusWithDesign),
			CompletedHUs:    int(r.CompletedHus),
			TotalTasks:      int(r.TotalTasks),
			CompletedTasks:  int(r.CompletedTasks),
			TaskPct:         numericToFloat(r.TaskPct),
		})
	}
	return out, nil
}

// AddCodeReference creates a code reference link.
func (s *Service) AddCodeReference(ctx context.Context, issueID uuid.UUID, filePath, repo, branch string) (*CodeRefSummary, error) {
	row, err := s.q().AddCodeReference(ctx, traceabilitydb.AddCodeReferenceParams{
		IssueID:  issueID,
		FilePath: filePath,
		Repo:     repo,
		Branch:   nullStr(branch),
	})
	if err != nil {
		return nil, fmt.Errorf("add code reference: %w", err)
	}
	return &CodeRefSummary{ID: row.ID, FilePath: row.FilePath, Repo: row.Repo, Branch: row.Branch}, nil
}

// RemoveCodeReference removes a code reference by ID.
func (s *Service) RemoveCodeReference(ctx context.Context, refID uuid.UUID) error {
	if err := s.q().RemoveCodeReference(ctx, refID); err != nil {
		return fmt.Errorf("remove code reference: %w", err)
	}
	return nil
}

// numericToFloat convierte un pgtype.Numeric a float64; 0 si NULL/inválido.
func numericToFloat(n pgtype.Numeric) float64 {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

func nullStr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
