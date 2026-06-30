// Tools MCP de project_repositories (REQ-42): permitir al LLM registrar y
// consultar los remotos conocidos por proyecto. Cuando hay ambiguedad
// (>1 remoto, ninguno marcado default), el LLM debe consultar antes de
// pushear.
package mcpserver

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/auth/apikey"
	projsvc "nunezlagos/domain/internal/service/project"
	projectreposvc "nunezlagos/domain/internal/service/projectrepo"
)

type projectRepoService interface {
	Add(ctx context.Context, in projectreposvc.AddInput) (*projectreposvc.Repo, error)
	List(ctx context.Context, orgID, projectID uuid.UUID) ([]*projectreposvc.Repo, error)
	SetDefault(ctx context.Context, orgID, id uuid.UUID) (*projectreposvc.Repo, error)
	Delete(ctx context.Context, orgID, id uuid.UUID) error
}

type projectRepoProjectGetter interface {
	GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*projsvc.Project, error)
}

type projectRepoHandlers struct {
	repos     projectRepoService
	projects  projectRepoProjectGetter
	principal *apikey.Principal
}

func registerProjectRepoTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	h := &projectRepoHandlers{
		repos:     deps.ProjectRepos,
		projects:  deps.Projects,
		principal: deps.Principal,
	}
	rls := func(fn mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withOrgTxHandler(&deps, fn)
	}
	return []mcpgo.ServerTool{
		{Tool: toolProjectRepoAdd(), Handler: wrap.Wrap("domain_project_repo_add", rls(h.handleProjectRepoAdd))},
		{Tool: toolProjectRepoList(), Handler: wrap.Wrap("domain_project_repo_list", rls(h.handleProjectRepoList))},
		{Tool: toolProjectRepoSetDefault(), Handler: wrap.Wrap("domain_project_repo_set_default", rls(h.handleProjectRepoSetDefault))},
		{Tool: toolProjectRepoDelete(), Handler: wrap.Wrap("domain_project_repo_delete", rls(h.handleProjectRepoDelete))},
	}
}

func toolProjectRepoAdd() mcp.Tool {
	return mcp.NewTool("domain_project_repo_add",
		mcp.WithDescription("Registra un remoto (origin/upstream/mirror/etc.) para un proyecto. El primer remoto agregado queda is_default automaticamente. workflow: merge|pr|mr|trunk_based|rebase. kind: github|gitlab|bitbucket|gitea|other."),
		mcp.WithString("project_slug", mcp.Description("Slug del proyecto al que pertenece este remoto."), mcp.Required()),
		mcp.WithString("name", mcp.Description("Alias del remoto (origin, upstream, mirror, ...). Unico por proyecto."), mcp.Required()),
		mcp.WithString("url", mcp.Description("URL del remoto (https:// o git@...)."), mcp.Required()),
		mcp.WithString("branch_default", mcp.Description("Rama principal en este remoto (main, master, services, ...). Opcional pero recomendado.")),
		mcp.WithString("kind", mcp.Description("Provider: github|gitlab|bitbucket|gitea|other.")),
		mcp.WithString("workflow", mcp.Description("Como se llevan cambios a este remoto: merge|pr|mr|trunk_based|rebase. Vacio = sin opinion.")),
		mcp.WithBoolean("is_default", mcp.Description("Forzarlo como default del proyecto (solo 1 default por proyecto).")),
		mcp.WithString("notes", mcp.Description("Notas libres (ej. 'sin push directo, requiere review obligatoria').")),
	)
}

func toolProjectRepoList() mcp.Tool {
	return mcp.NewTool("domain_project_repo_list",
		mcp.WithDescription("Lista los remotos registrados de un proyecto, ordenados por is_default DESC, nombre ASC. Llamar antes de pushear si hay ambiguedad o si el LLM no sabe el remoto."),
		mcp.WithString("project_slug", mcp.Description("Slug del proyecto."), mcp.Required()),
	)
}

func toolProjectRepoSetDefault() mcp.Tool {
	return mcp.NewTool("domain_project_repo_set_default",
		mcp.WithDescription("Marca un remoto como default del proyecto. Limpia el default previo automaticamente."),
		mcp.WithString("repo_id", mcp.Description("UUID del project_repository."), mcp.Required()),
	)
}

func toolProjectRepoDelete() mcp.Tool {
	return mcp.NewTool("domain_project_repo_delete",
		mcp.WithDescription("Soft-delete de un remoto del proyecto. Si era default, el proyecto queda sin default — el LLM tendra que preguntar."),
		mcp.WithString("repo_id", mcp.Description("UUID del project_repository."), mcp.Required()),
	)
}

func (h *projectRepoHandlers) requireDeps() error {
	if h.principal == nil {
		return fmt.Errorf("no authenticated principal")
	}
	if h.repos == nil {
		return fmt.Errorf("project_repositories service not configured")
	}
	if h.projects == nil {
		return fmt.Errorf("projects service not configured")
	}
	return nil
}

func (h *projectRepoHandlers) resolveProjectIDFromSlug(ctx context.Context, orgID uuid.UUID, slug string) (uuid.UUID, error) {
	proj, err := h.projects.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return uuid.Nil, fmt.Errorf("project '%s' not found: %w", slug, err)
	}
	return proj.ID, nil
}

func (h *projectRepoHandlers) handleProjectRepoAdd(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	args := req.GetArguments()
	slug, _ := args["project_slug"].(string)
	name, _ := args["name"].(string)
	url, _ := args["url"].(string)
	if slug == "" || name == "" || url == "" {
		return mcp.NewToolResultError("project_slug, name y url requeridos"), nil
	}
	projID, err := h.resolveProjectIDFromSlug(ctx, orgID, slug)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	in := projectreposvc.AddInput{
		ProjectID: projID,
		Name:      name,
		URL:       url,
	}
	if v, ok := args["branch_default"].(string); ok {
		in.BranchDefault = v
	}
	if v, ok := args["kind"].(string); ok {
		in.Kind = v
	}
	if v, ok := args["workflow"].(string); ok {
		in.Workflow = v
	}
	if v, ok := args["is_default"].(bool); ok {
		in.IsDefault = v
	}
	if v, ok := args["notes"].(string); ok {
		in.Notes = v
	}
	repo, err := h.repos.Add(ctx, in)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("add failed: %v", err)), nil
	}
	return toolResultJSON(repo)
}

func (h *projectRepoHandlers) handleProjectRepoList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	args := req.GetArguments()
	slug, _ := args["project_slug"].(string)
	if slug == "" {
		return mcp.NewToolResultError("project_slug requerido"), nil
	}
	projID, err := h.resolveProjectIDFromSlug(ctx, orgID, slug)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	list, err := h.repos.List(ctx, orgID, projID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list failed: %v", err)), nil
	}

	ambiguous := len(list) > 1
	for _, r := range list {
		if r.IsDefault {
			ambiguous = false
			break
		}
	}
	return toolResultJSON(map[string]any{
		"repos":     list,
		"total":     len(list),
		"ambiguous": ambiguous,
	})
}

func (h *projectRepoHandlers) handleProjectRepoSetDefault(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	args := req.GetArguments()
	idStr, _ := args["repo_id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("repo_id invalido"), nil
	}
	repo, err := h.repos.SetDefault(ctx, orgID, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("set_default failed: %v", err)), nil
	}
	return toolResultJSON(repo)
}

func (h *projectRepoHandlers) handleProjectRepoDelete(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	args := req.GetArguments()
	idStr, _ := args["repo_id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("repo_id invalido"), nil
	}
	if err := h.repos.Delete(ctx, orgID, id); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("delete failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"id": id.String(), "deleted": true})
}
