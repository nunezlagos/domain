// domain_openspec_* — round-trip de specs SDD entre la DB y el repo en el
// layout openspec oficial (change-céntrico).
//
//	export  DB→repo   renderiza issues como changes/<slug>/{proposal,design,tasks,specs,.openspec.yaml}
//	status  audita    compara hashes repo↔DB y reporta drift por archivo
//	apply   repo→DB    parsea los .md editados y persiste (proposal/design versionados)
//
// El server nunca escribe en el filesystem del cliente: export devuelve
// {path: contenido} y el cliente MCP los escribe; apply recibe los archivos
// editados. Mismo patrón que domain_project_index_*.
package mcpserver

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/auth/apikey"
	issuesvc "nunezlagos/domain/internal/service/issue"
	specsvc "nunezlagos/domain/internal/service/spec"
	tasksvc "nunezlagos/domain/internal/service/task"
	"nunezlagos/domain/internal/store/txctx"
)

type specReader interface {
	GetLatestProposal(ctx context.Context, issueID uuid.UUID) (*specsvc.Proposal, error)
	GetLatestDesign(ctx context.Context, issueID uuid.UUID) (*specsvc.Design, error)
}

type specWriter interface {
	CreateProposal(ctx context.Context, issueID uuid.UUID, intention, scope, approach, risks, testingNotes string) (*specsvc.Proposal, error)
	CreateDesign(ctx context.Context, issueID uuid.UUID, proposalID *uuid.UUID, archDecisions, alternatives, dataFlow, tddPlan, risksMitigation string) (*specsvc.Design, error)
}

type taskReader interface {
	ListTasks(ctx context.Context, issueID uuid.UUID) ([]tasksvc.Task, error)
}

type taskWriter interface {
	UpdateTaskStatus(ctx context.Context, taskID uuid.UUID, newStatus, completedBy string) (*tasksvc.Task, error)
}

type issueReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*issuesvc.Issue, error)
}

type issueWriter interface {
	Update(ctx context.Context, slug string, title, description, status, priority *string) (*issuesvc.Issue, error)
	AddScenario(ctx context.Context, huSlug string, sc issuesvc.Scenario) (*issuesvc.Scenario, error)
	RemoveScenario(ctx context.Context, scenarioID uuid.UUID) error
}

type openspecHandlers struct {
	issuesR   issueReader
	issuesW   issueWriter
	specR     specReader
	specW     specWriter
	tasksR    taskReader
	tasksW    taskWriter
	projects  indexProjectGetter
	pool      *pgxpool.Pool
	principal *apikey.Principal
}

func (h *openspecHandlers) q(ctx context.Context) interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
} {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return tx
	}
	return h.pool
}

func registerOpenspecTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	h := &openspecHandlers{
		issuesR:   deps.IssueSvc,
		issuesW:   deps.IssueSvc,
		specR:     deps.Spec,
		specW:     deps.Spec,
		tasksR:    deps.Tasks,
		tasksW:    deps.Tasks,
		projects:  deps.Projects,
		pool:      deps.Pool,
		principal: deps.Principal,
	}
	rls := func(fn mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withOrgTxHandler(&deps, fn)
	}
	return []mcpgo.ServerTool{
		{Tool: toolOpenspecExport(), Handler: wrap.Wrap("domain_openspec_export", rls(h.handleExport))},
		{Tool: toolOpenspecStatus(), Handler: wrap.Wrap("domain_openspec_status", rls(h.handleStatus))},
		{Tool: toolOpenspecApply(), Handler: wrap.Wrap("domain_openspec_apply", rls(h.handleApply))},
	}
}

func toolOpenspecExport() mcp.Tool {
	return mcp.NewTool("domain_openspec_export",
		mcp.WithDescription("Renderiza los issues SDD del proyecto como un árbol openspec oficial (changes/<slug>/{proposal.md,design.md,tasks.md,specs/<slug>/spec.md,.openspec.yaml}). Devuelve {path: contenido}; vos (LLM) escribís cada archivo con tu tool Write. El server NO toca el filesystem. Issues proposed/active van bajo changes/; implemented/archived bajo changes/archive/<fecha>-<slug>/."),
		mcp.WithString("project_slug", mcp.Description("Proyecto a exportar"), mcp.Required()),
		mcp.WithString("scope", mcp.Description("active = solo proposed+active (default). all = incluye archived/implemented.")),
	)
}

func toolOpenspecStatus() mcp.Tool {
	return mcp.NewTool("domain_openspec_status",
		mcp.WithDescription("Auditoría/diff repo↔DB. Pasale los archivos actuales del repo ({path, content}, incluí los .openspec.yaml). El server compara 3 hashes por change: el del export (en .openspec.yaml), el del repo actual, y el de la DB actual. Reporta por change: clean | repo_modified (editaste, seguro de aplicar) | db_modified (repo desactualizado, re-exportá) | conflict (cambió en ambos)."),
		mcp.WithArray("files", mcp.Description("Array de {path: 'relativo al repo', content: 'texto'}"), mcp.Required()),
	)
}

func toolOpenspecApply() mcp.Tool {
	return mcp.NewTool("domain_openspec_apply",
		mcp.WithDescription("Persiste en la DB los .md editados en el repo. Pasale los archivos del/los change(s) ({path, content}, incluí .openspec.yaml). Por cada change: proposal.md/design.md que cambiaron crean NUEVA versión (no pisan historial); spec.md reemplaza los escenarios Gherkin; tasks.md sincroniza estado por checkbox (vía marcador de id); .openspec.yaml status actualiza el estado del issue. Si la DB cambió desde el export (conflict) aborta el change salvo force=true."),
		mcp.WithArray("files", mcp.Description("Array de {path, content} de uno o más changes"), mcp.Required()),
		mcp.WithBoolean("force", mcp.Description("Aplicar aunque haya conflict (la DB cambió desde el export). Default false.")),
	)
}

func (h *openspecHandlers) handleExport(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	orgID, errResult := h.requireOrg()
	if errResult != nil {
		return errResult, nil
	}
	args := req.GetArguments()
	slug, _ := args["project_slug"].(string)
	if slug == "" {
		return mcp.NewToolResultError("project_slug requerido"), nil
	}
	proj, err := h.projects.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", slug)), nil
	}
	scope, _ := args["scope"].(string)
	rows, err := h.queryIssues(ctx, proj.ID, scope)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query issues: %v", err)), nil
	}
	changes := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		rendered, err := h.renderIssue(ctx, row)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("render issue %s: %v", row.slug, err)), nil
		}
		changes = append(changes, rendered)
	}
	return toolResultJSON(map[string]any{
		"project_slug": slug,
		"change_count": len(changes),
		"changes":      changes,
		"next_step":    "Escribí cada archivo de changes[].files (key=path, value=contenido) con tu tool Write. Después editás los .md y corrés domain_openspec_apply.",
	})
}

func (h *openspecHandlers) handleStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, errResult := h.requireOrg(); errResult != nil {
		return errResult, nil
	}
	byDir, errResult := groupFilesByChange(req)
	if errResult != nil {
		return errResult, nil
	}
	results := make([]map[string]any, 0, len(byDir))
	for dir, files := range byDir {
		results = append(results, h.statusForChange(ctx, dir, files))
	}
	return toolResultJSON(map[string]any{
		"change_count": len(results),
		"changes":      results,
	})
}

func (h *openspecHandlers) requireOrg() (uuid.UUID, *mcp.CallToolResult) {
	if h.principal == nil || h.projects == nil {
		return uuid.Nil, mcp.NewToolResultError("principal o projects service no configurado")
	}
	orgID, err := uuid.Parse(h.principal.OrganizationID)
	if err != nil {
		return uuid.Nil, mcp.NewToolResultError("invalid principal org_id")
	}
	return orgID, nil
}
