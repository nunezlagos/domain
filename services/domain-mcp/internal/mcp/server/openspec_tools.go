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
//
// La lógica de negocio (render/status/apply) vive en internal/service/openspec
// (Engine), compartida con el handler HTTP REST. Estos handlers son adaptadores
// MCP: extraen args, invocan el Engine y serializan la respuesta.
package mcpserver

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/auth/apikey"
	"nunezlagos/domain/internal/service/openspec"
)

type openspecHandlers struct {
	engine    *openspec.Engine
	projects  indexProjectGetter
	principal *apikey.Principal
}

func registerOpenspecTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	h := &openspecHandlers{
		engine: &openspec.Engine{
			IssuesR: deps.IssueSvc,
			IssuesW: deps.IssueSvc,
			SpecR:   deps.Spec,
			SpecW:   deps.Spec,
			TasksR:  deps.Tasks,
			TasksW:  deps.Tasks,
			Pool:    deps.Pool,
		},
		projects:  deps.Projects,
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
		mcp.WithDescription("Renderiza los issues SDD del proyecto como un árbol openspec oficial (changes/<slug>/{proposal.md,design.md,tasks.md,specs/<slug>/spec.md,.openspec.yaml}). Devuelve {path: contenido}; el LLM escribe cada archivo con su tool Write. El server NO toca el filesystem. Issues proposed/active van bajo changes/; implemented/archived bajo changes/archive/<fecha>-<slug>/."),
		mcp.WithString("project_slug", mcp.Description("Proyecto a exportar"), mcp.Required()),
		mcp.WithString("scope", mcp.Description("active = solo proposed+active (default). all = incluye archived/implemented.")),
	)
}

func toolOpenspecStatus() mcp.Tool {
	return mcp.NewTool("domain_openspec_status",
		mcp.WithDescription("Auditoría/diff repo↔DB. Pasa los archivos actuales del repo ({path, content}, incluye los .openspec.yaml). El server compara 3 hashes por change: el del export (en .openspec.yaml), el del repo actual, y el de la DB actual. Reporta por change: clean | repo_modified (editaste, seguro de aplicar) | db_modified (repo desactualizado, re-exporta) | conflict (cambió en ambos)."),
		mcp.WithArray("files", mcp.Description("Array de {path: 'relativo al repo', content: 'texto'}"), mcp.Required()),
	)
}

func toolOpenspecApply() mcp.Tool {
	return mcp.NewTool("domain_openspec_apply",
		mcp.WithDescription("Persiste en la DB los .md editados en el repo. Pasa los archivos del/los change(s) ({path, content}, incluye .openspec.yaml). Por cada change: proposal.md/design.md que cambiaron crean NUEVA versión (no pisan historial); spec.md reemplaza los escenarios Gherkin; tasks.md sincroniza estado por checkbox (vía marcador de id); .openspec.yaml status actualiza el estado del issue. Si la DB cambió desde el export (conflict) aborta el change salvo force=true."),
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
	changes, err := h.engine.Export(ctx, proj.ID, scope)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	out := make([]map[string]any, 0, len(changes))
	for _, c := range changes {
		out = append(out, map[string]any{
			"issue_slug": c.IssueSlug, "dir": c.Dir,
			"status": c.Status, "files": c.Files,
		})
	}
	return toolResultJSON(map[string]any{
		"project_slug": slug,
		"change_count": len(out),
		"changes":      out,
		"next_step":    "Escribe cada archivo de changes[].files (key=path, value=contenido) con tu tool Write. Después editas los .md y ejecutas domain_openspec_apply.",
	})
}

func (h *openspecHandlers) handleStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, errResult := h.requireOrg(); errResult != nil {
		return errResult, nil
	}
	files, errResult := openspecFilesArg(req)
	if errResult != nil {
		return errResult, nil
	}
	results := h.engine.Status(ctx, files)
	out := make([]map[string]any, 0, len(results))
	for _, r := range results {
		out = append(out, statusResultMap(r))
	}
	return toolResultJSON(map[string]any{
		"change_count": len(out),
		"changes":      out,
	})
}

func (h *openspecHandlers) handleApply(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if _, errResult := h.requireOrg(); errResult != nil {
		return errResult, nil
	}
	files, errResult := openspecFilesArg(req)
	if errResult != nil {
		return errResult, nil
	}
	force, _ := req.GetArguments()["force"].(bool)
	results := h.engine.Apply(ctx, files, force, h.principalUserID())
	out := make([]map[string]any, 0, len(results))
	for _, r := range results {
		out = append(out, applyResultMap(r))
	}
	return toolResultJSON(map[string]any{
		"change_count": len(out),
		"changes":      out,
	})
}

// openspecFilesArg extrae el array files ({path, content}) de la request MCP.
func openspecFilesArg(req mcp.CallToolRequest) ([]openspec.File, *mcp.CallToolResult) {
	rawFiles, _ := req.GetArguments()["files"].([]any)
	if len(rawFiles) == 0 {
		return nil, mcp.NewToolResultError("files requerido (no vacío)")
	}
	files := make([]openspec.File, 0, len(rawFiles))
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
		files = append(files, openspec.File{Path: path, Content: content})
	}
	return files, nil
}

func statusResultMap(r openspec.StatusResult) map[string]any {
	m := map[string]any{"dir": r.Dir, "verdict": r.Verdict}
	if r.IssueSlug != "" {
		m["issue_slug"] = r.IssueSlug
	}
	if r.Reason != "" {
		m["reason"] = r.Reason
	}
	if r.Files != nil {
		m["files"] = r.Files
	}
	return m
}

func applyResultMap(r openspec.ApplyResult) map[string]any {
	m := map[string]any{"dir": r.Dir}
	if r.Error != "" {
		m["error"] = r.Error
		return m
	}
	m["issue_slug"] = r.IssueSlug
	m["applied"] = r.Applied
	m["conflicts"] = r.Conflicts
	return m
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

func (h *openspecHandlers) principalUserID() string {
	if h.principal == nil {
		return "openspec-sync"
	}
	return h.principal.UserID
}
