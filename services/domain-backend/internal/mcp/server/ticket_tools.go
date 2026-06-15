// REQ-51 — Tools MCP para sistema de tickets internos por proyecto.
// BD = source of truth. Tickets se sincronizan opcionalmente con
// Jira/GitHub/GitLab/Linear/Azure DevOps via domain_ticket_link_external.
package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	ticketsvc "nunezlagos/domain/internal/service/ticket"
)

func registerTicketTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	rls := func(h mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withOrgTxHandler(&deps, h)
	}
	return []mcpgo.ServerTool{
		{Tool: toolTicketCreate(), Handler: wrap.Wrap("domain_ticket_create", rls(deps.handleTicketCreate))},
		{Tool: toolTicketGet(), Handler: wrap.Wrap("domain_ticket_get", rls(deps.handleTicketGet))},
		{Tool: toolTicketList(), Handler: wrap.Wrap("domain_ticket_list", rls(deps.handleTicketList))},
		{Tool: toolTicketUpdate(), Handler: wrap.Wrap("domain_ticket_update", rls(deps.handleTicketUpdate))},
		{Tool: toolTicketChangeStatus(), Handler: wrap.Wrap("domain_ticket_change_status", rls(deps.handleTicketChangeStatus))},
		{Tool: toolTicketDelete(), Handler: wrap.Wrap("domain_ticket_delete", rls(deps.handleTicketDelete))},
		{Tool: toolTicketCommentAdd(), Handler: wrap.Wrap("domain_ticket_comment_add", rls(deps.handleTicketCommentAdd))},
		{Tool: toolTicketCommentList(), Handler: wrap.Wrap("domain_ticket_comment_list", rls(deps.handleTicketCommentList))},
		{Tool: toolTicketStatusHistory(), Handler: wrap.Wrap("domain_ticket_status_history", rls(deps.handleTicketStatusHistory))},
		{Tool: toolTicketLinkExternal(), Handler: wrap.Wrap("domain_ticket_link_external", rls(deps.handleTicketLinkExternal))},
		{Tool: toolTicketLinkIssue(), Handler: wrap.Wrap("domain_ticket_link_issue", rls(deps.handleTicketLinkIssue))},
		{Tool: toolTicketLinkExternalBulk(), Handler: wrap.Wrap("domain_ticket_link_external_bulk", rls(deps.handleTicketLinkExternalBulk))},
	}
}

func toolTicketCreate() mcp.Tool {
	return mcp.NewTool("domain_ticket_create",
		mcp.WithDescription("Crea un ticket interno en un proyecto. Auto-genera key (PROJ-1, PROJ-2 derivado del project slug). issue_type: bug|feature|requirement|task|epic|improvement|spike. priority: trivial|low|medium|high|critical. status arranca en 'backlog'. Para vincular con Jira/etc usá domain_ticket_link_external después."),
		mcp.WithString("project_slug", mcp.Description("Proyecto al que pertenece el ticket"), mcp.Required()),
		mcp.WithString("title", mcp.Description("Título corto"), mcp.Required()),
		mcp.WithString("description_md", mcp.Description("Descripción markdown")),
		mcp.WithString("issue_type", mcp.Description("bug|feature|requirement|task|epic|improvement|spike. Default: task")),
		mcp.WithString("priority", mcp.Description("trivial|low|medium|high|critical. Default: medium")),
		mcp.WithString("client_slug", mcp.Description("Si el ticket pertenece a un cliente/mandante específico del proyecto")),
		mcp.WithString("assignee_id", mcp.Description("UUID del usuario asignado (opcional)")),
		mcp.WithArray("labels", mcp.Description("Tags libres ej: ['urgente','frontend']")),
		mcp.WithString("parent_id", mcp.Description("UUID de epic/story padre (opcional)")),
		mcp.WithNumber("estimated_hours", mcp.Description("Estimación en horas (decimal)")),
		mcp.WithString("due_date", mcp.Description("YYYY-MM-DD opcional")),
		mcp.WithString("external_provider", mcp.Description("REQ-58: si el ticket ya está en Jira/etc, vincular en el mismo INSERT (jira|github|gitlab|linear|azure_devops).")),
		mcp.WithString("external_id", mcp.Description("Key externo (ej: MPS-12). Requiere external_provider.")),
		mcp.WithString("external_url", mcp.Description("URL al ticket externo. Opcional.")),
	)
}

func toolTicketGet() mcp.Tool {
	return mcp.NewTool("domain_ticket_get",
		mcp.WithDescription("Obtiene un ticket por id o por key (ej: ACMEWEB-15 + project_slug). Si pasás ambos, gana id."),
		mcp.WithString("id", mcp.Description("UUID del ticket")),
		mcp.WithString("project_slug", mcp.Description("Si vas a buscar por key")),
		mcp.WithString("key", mcp.Description("Ej: ACMEWEB-15")),
	)
}

func toolTicketList() mcp.Tool {
	return mcp.NewTool("domain_ticket_list",
		mcp.WithDescription("Lista tickets filtrados por proyecto/status/type/priority/assignee/label/parent/búsqueda. Default ordena por updated_at DESC."),
		mcp.WithString("project_slug", mcp.Description("Filtrar por proyecto")),
		mcp.WithString("status", mcp.Description("backlog|todo|in_progress|in_review|blocked|done|cancelled")),
		mcp.WithString("issue_type", mcp.Description("bug|feature|...")),
		mcp.WithString("priority", mcp.Description("trivial|low|medium|high|critical")),
		mcp.WithString("assignee_id", mcp.Description("UUID del assignee")),
		mcp.WithString("reporter_id", mcp.Description("UUID del reporter")),
		mcp.WithString("parent_id", mcp.Description("UUID del epic/story padre — para listar subtasks")),
		mcp.WithString("label", mcp.Description("Filtrar por una label específica")),
		mcp.WithString("query", mcp.Description("Full-text search sobre title+description")),
		mcp.WithNumber("limit", mcp.Description("Default 50, max 200")),
		mcp.WithNumber("offset", mcp.Description("Default 0")),
	)
}

func toolTicketUpdate() mcp.Tool {
	return mcp.NewTool("domain_ticket_update",
		mcp.WithDescription("Update parcial. Solo los campos provistos se modifican. Para cambiar status usá domain_ticket_change_status (registra history). Para des-asignar pasá assignee_id='00000000-0000-0000-0000-000000000000'."),
		mcp.WithString("id", mcp.Description("UUID del ticket"), mcp.Required()),
		mcp.WithString("title"),
		mcp.WithString("description_md"),
		mcp.WithString("issue_type"),
		mcp.WithString("priority"),
		mcp.WithString("assignee_id"),
		mcp.WithArray("labels"),
		mcp.WithString("parent_id"),
		mcp.WithNumber("estimated_hours"),
		mcp.WithNumber("actual_hours"),
		mcp.WithString("due_date", mcp.Description("YYYY-MM-DD")),
	)
}

func toolTicketChangeStatus() mcp.Tool {
	return mcp.NewTool("domain_ticket_change_status",
		mcp.WithDescription("Transición de status. Registra entry en status_history con quién y por qué. Auto-setea started_at en primer in_progress y completed_at en done/cancelled."),
		mcp.WithString("id", mcp.Description("UUID del ticket"), mcp.Required()),
		mcp.WithString("to_status", mcp.Description("backlog|todo|in_progress|in_review|blocked|done|cancelled"), mcp.Required()),
		mcp.WithString("note", mcp.Description("Razón / comentario de la transición")),
	)
}

func toolTicketDelete() mcp.Tool {
	return mcp.NewTool("domain_ticket_delete",
		mcp.WithDescription("Soft-delete. Preserva comments + status_history en audit."),
		mcp.WithString("id", mcp.Description("UUID"), mcp.Required()),
	)
}

func toolTicketCommentAdd() mcp.Tool {
	return mcp.NewTool("domain_ticket_comment_add",
		mcp.WithDescription("Agrega un comentario markdown al ticket. Author = principal del MCP. Los comments del sync externo (Jira) se importan con external_id; este tool es para comments internos."),
		mcp.WithString("ticket_id", mcp.Description("UUID del ticket"), mcp.Required()),
		mcp.WithString("body_md", mcp.Description("Contenido markdown"), mcp.Required()),
	)
}

func toolTicketCommentList() mcp.Tool {
	return mcp.NewTool("domain_ticket_comment_list",
		mcp.WithDescription("Lista comments del ticket en orden cronológico ASC."),
		mcp.WithString("ticket_id", mcp.Description("UUID"), mcp.Required()),
	)
}

func toolTicketStatusHistory() mcp.Tool {
	return mcp.NewTool("domain_ticket_status_history",
		mcp.WithDescription("Devuelve la historia de transiciones de status (audit). Útil para calcular cycle time, lead time, ver quién bloqueó qué."),
		mcp.WithString("ticket_id", mcp.Description("UUID"), mcp.Required()),
	)
}

func toolTicketLinkExternal() mcp.Tool {
	return mcp.NewTool("domain_ticket_link_external",
		mcp.WithDescription("Vincula el ticket con uno de un sistema externo (Jira/GitHub/GitLab/Linear/Azure DevOps). Marca external_synced_at = NOW(). Pasando provider='' lo des-vincula."),
		mcp.WithString("id", mcp.Description("UUID del ticket"), mcp.Required()),
		mcp.WithString("provider", mcp.Description("jira|github|gitlab|linear|azure_devops")),
		mcp.WithString("external_id", mcp.Description("Clave en el sistema externo (ej: PROJ-123)")),
		mcp.WithString("external_url", mcp.Description("URL del ticket externo")),
	)
}

// --- handlers ---

func (d *Deps) requireTicketDeps() error {
	if d.Principal == nil {
		return fmt.Errorf("no authenticated principal")
	}
	if d.Tickets == nil {
		return fmt.Errorf("tickets service not configured")
	}
	if d.Projects == nil {
		return fmt.Errorf("projects service not configured")
	}
	return nil
}

func parseDateYMD(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", strings.TrimSpace(s))
	if err != nil {
		return nil
	}
	return &t
}

func (d *Deps) handleTicketCreate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := d.requireTicketDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := req.GetArguments()
	projSlug, _ := args["project_slug"].(string)
	title, _ := args["title"].(string)
	if projSlug == "" || title == "" {
		return mcp.NewToolResultError("project_slug y title requeridos"), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	userID, _ := uuid.Parse(d.Principal.UserID)
	proj, perr := d.Projects.GetBySlug(ctx, orgID, projSlug)
	if perr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", projSlug)), nil
	}

	in := ticketsvc.CreateInput{
		OrganizationID: orgID,
		ProjectID:      proj.ID,
		ProjectSlug:    projSlug,
		Title:          title,
		ReporterID:     userID,
		Labels:         []string{},
	}
	if v, ok := args["description_md"].(string); ok {
		in.DescriptionMD = v
	}
	if v, ok := args["issue_type"].(string); ok {
		in.IssueType = v
	}
	if v, ok := args["priority"].(string); ok {
		in.Priority = v
	}
	if v, ok := args["client_slug"].(string); ok && v != "" && d.Clients != nil {
		if cl, _ := d.Clients.Get(ctx, orgID, v); cl != nil {
			cid := cl.ID
			in.ClientID = &cid
		}
	}
	if v, ok := args["assignee_id"].(string); ok && v != "" {
		if aid, err := uuid.Parse(v); err == nil {
			in.AssigneeID = &aid
		}
	}
	if v, ok := args["labels"].([]any); ok {
		for _, l := range v {
			if s, ok := l.(string); ok && s != "" {
				in.Labels = append(in.Labels, s)
			}
		}
	}
	if v, ok := args["parent_id"].(string); ok && v != "" {
		if pid, err := uuid.Parse(v); err == nil {
			in.ParentID = &pid
		}
	}
	if v, ok := args["estimated_hours"].(float64); ok && v > 0 {
		in.EstimatedHours = &v
	}
	if v, ok := args["due_date"].(string); ok && v != "" {
		in.DueDate = parseDateYMD(v)
	}
	if v, ok := args["external_provider"].(string); ok {
		in.ExternalProvider = v
	}
	if v, ok := args["external_id"].(string); ok {
		in.ExternalID = v
	}
	if v, ok := args["external_url"].(string); ok {
		in.ExternalURL = v
	}

	t, err := d.Tickets.Create(ctx, in)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create ticket failed: %v", err)), nil
	}
	return toolResultJSON(t)
}

func (d *Deps) handleTicketGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := d.requireTicketDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	args := req.GetArguments()
	if idStr, _ := args["id"].(string); idStr != "" {
		id, err := uuid.Parse(idStr)
		if err != nil {
			return mcp.NewToolResultError("id inválido"), nil
		}
		t, err := d.Tickets.Get(ctx, orgID, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("ticket not found: %v", err)), nil
		}
		return toolResultJSON(t)
	}
	projSlug, _ := args["project_slug"].(string)
	key, _ := args["key"].(string)
	if projSlug == "" || key == "" {
		return mcp.NewToolResultError("pasá id o (project_slug + key)"), nil
	}
	proj, perr := d.Projects.GetBySlug(ctx, orgID, projSlug)
	if perr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", projSlug)), nil
	}
	t, err := d.Tickets.GetByKey(ctx, orgID, proj.ID, key)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("ticket %s not found", key)), nil
	}
	return toolResultJSON(t)
}

func (d *Deps) handleTicketList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := d.requireTicketDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	args := req.GetArguments()
	filter := ticketsvc.ListFilter{}
	if v, ok := args["project_slug"].(string); ok && v != "" {
		if proj, perr := d.Projects.GetBySlug(ctx, orgID, v); perr == nil {
			pid := proj.ID
			filter.ProjectID = &pid
		}
	}
	if v, ok := args["status"].(string); ok {
		filter.Status = v
	}
	if v, ok := args["issue_type"].(string); ok {
		filter.IssueType = v
	}
	if v, ok := args["priority"].(string); ok {
		filter.Priority = v
	}
	if v, ok := args["assignee_id"].(string); ok && v != "" {
		if aid, err := uuid.Parse(v); err == nil {
			filter.AssigneeID = &aid
		}
	}
	if v, ok := args["reporter_id"].(string); ok && v != "" {
		if rid, err := uuid.Parse(v); err == nil {
			filter.ReporterID = &rid
		}
	}
	if v, ok := args["parent_id"].(string); ok && v != "" {
		if pid, err := uuid.Parse(v); err == nil {
			filter.ParentID = &pid
		}
	}
	if v, ok := args["label"].(string); ok {
		filter.Label = v
	}
	if v, ok := args["query"].(string); ok {
		filter.Query = v
	}
	if v, ok := args["limit"].(float64); ok {
		filter.Limit = int(v)
	}
	if v, ok := args["offset"].(float64); ok {
		filter.Offset = int(v)
	}
	list, total, err := d.Tickets.List(ctx, orgID, filter)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"tickets": list, "total": total})
}

func (d *Deps) handleTicketUpdate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := d.requireTicketDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id inválido"), nil
	}
	in := ticketsvc.UpdateInput{}
	if v, ok := args["title"].(string); ok {
		in.Title = &v
	}
	if v, ok := args["description_md"].(string); ok {
		in.DescriptionMD = &v
	}
	if v, ok := args["issue_type"].(string); ok {
		in.IssueType = &v
	}
	if v, ok := args["priority"].(string); ok {
		in.Priority = &v
	}
	if v, ok := args["assignee_id"].(string); ok {
		if aid, err := uuid.Parse(v); err == nil {
			in.AssigneeID = &aid
		}
	}
	if v, ok := args["labels"].([]any); ok {
		labels := []string{}
		for _, l := range v {
			if s, ok := l.(string); ok {
				labels = append(labels, s)
			}
		}
		in.Labels = &labels
	}
	if v, ok := args["parent_id"].(string); ok {
		if pid, err := uuid.Parse(v); err == nil {
			in.ParentID = &pid
		}
	}
	if v, ok := args["estimated_hours"].(float64); ok {
		in.EstimatedHours = &v
	}
	if v, ok := args["actual_hours"].(float64); ok {
		in.ActualHours = &v
	}
	if v, ok := args["due_date"].(string); ok && v != "" {
		in.DueDate = parseDateYMD(v)
	}
	t, err := d.Tickets.Update(ctx, orgID, id, in)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("update failed: %v", err)), nil
	}
	return toolResultJSON(t)
}

func (d *Deps) handleTicketChangeStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := d.requireTicketDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	userID, _ := uuid.Parse(d.Principal.UserID)
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id inválido"), nil
	}
	toStatus, _ := args["to_status"].(string)
	if toStatus == "" {
		return mcp.NewToolResultError("to_status requerido"), nil
	}
	note, _ := args["note"].(string)
	t, err := d.Tickets.ChangeStatus(ctx, orgID, id, toStatus, userID, note)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("change_status failed: %v", err)), nil
	}
	return toolResultJSON(t)
}

func (d *Deps) handleTicketDelete(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := d.requireTicketDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id inválido"), nil
	}
	if err := d.Tickets.Delete(ctx, orgID, id); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("delete failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"id": id.String(), "deleted": true})
}

func (d *Deps) handleTicketCommentAdd(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := d.requireTicketDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	userID, _ := uuid.Parse(d.Principal.UserID)
	args := req.GetArguments()
	idStr, _ := args["ticket_id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("ticket_id inválido"), nil
	}
	body, _ := args["body_md"].(string)
	c, err := d.Tickets.AddComment(ctx, id, userID, body)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("comment failed: %v", err)), nil
	}
	return toolResultJSON(c)
}

func (d *Deps) handleTicketCommentList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := d.requireTicketDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := req.GetArguments()
	idStr, _ := args["ticket_id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("ticket_id inválido"), nil
	}
	out, err := d.Tickets.ListComments(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list comments failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"comments": out, "total": len(out)})
}

func (d *Deps) handleTicketStatusHistory(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := d.requireTicketDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := req.GetArguments()
	idStr, _ := args["ticket_id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("ticket_id inválido"), nil
	}
	hist, err := d.Tickets.StatusHistory(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("history failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"history": hist, "total": len(hist)})
}

func (d *Deps) handleTicketLinkExternal(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := d.requireTicketDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id inválido"), nil
	}
	provider, _ := args["provider"].(string)
	if provider == "" {
		if err := d.Tickets.UnlinkExternal(ctx, orgID, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("unlink failed: %v", err)), nil
		}
		return toolResultJSON(map[string]any{"id": id.String(), "unlinked": true})
	}
	extID, _ := args["external_id"].(string)
	extURL, _ := args["external_url"].(string)
	t, err := d.Tickets.LinkExternal(ctx, orgID, id, ticketsvc.ExternalLink{
		Provider: provider, ID: extID, URL: extURL,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("link failed: %v", err)), nil
	}
	return toolResultJSON(t)
}

func toolTicketLinkIssue() mcp.Tool {
	return mcp.NewTool("domain_ticket_link_issue",
		mcp.WithDescription("Vincula un ticket operativo con una HU/issue del workflow SDD (REQ-56). Pasar issue_id='' para desvincular. Útil cuando el ticket implementa una HU formal con Gherkin scenarios."),
		mcp.WithString("ticket_id", mcp.Description("UUID del ticket"), mcp.Required()),
		mcp.WithString("issue_id", mcp.Description("UUID del issue (issues.id) o vacío para desvincular")),
	)
}

func (d *Deps) handleTicketLinkIssue(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := d.requireTicketDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	args := req.GetArguments()
	tStr, _ := args["ticket_id"].(string)
	tID, err := uuid.Parse(tStr)
	if err != nil {
		return mcp.NewToolResultError("ticket_id inválido"), nil
	}
	var issuePtr *uuid.UUID
	if iStr, _ := args["issue_id"].(string); iStr != "" {
		iID, perr := uuid.Parse(iStr)
		if perr != nil {
			return mcp.NewToolResultError("issue_id inválido"), nil
		}
		issuePtr = &iID
	}
	t, err := d.Tickets.LinkIssue(ctx, orgID, tID, issuePtr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("link_issue failed: %v", err)), nil
	}
	return toolResultJSON(t)
}

// REQ-58: bulk link (sync inicial Jira→domain)
func toolTicketLinkExternalBulk() mcp.Tool {
	return mcp.NewTool("domain_ticket_link_external_bulk",
		mcp.WithDescription("Vincula N tickets a sus externals en una operación. Para sync inicial cuando se enchufa Jira/GitHub/etc y hay que linkear tickets existentes en bulk. provider: jira|github|gitlab|linear|azure_devops. mappings: array de {ticket_key|ticket_id, external_id, external_url}."),
		mcp.WithString("project_slug", mcp.Description("Proyecto donde están los tickets"), mcp.Required()),
		mcp.WithString("provider", mcp.Description("Proveedor externo"), mcp.Required()),
		mcp.WithArray("mappings", mcp.Description("Array de mappings. Cada item: {ticket_key:'ACMEWEB-1', external_id:'MPS-12', external_url:'https://...'} o {ticket_id:UUID, ...}"), mcp.Required()),
	)
}

func (d *Deps) handleTicketLinkExternalBulk(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := d.requireTicketDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	args := req.GetArguments()
	slug, _ := args["project_slug"].(string)
	provider, _ := args["provider"].(string)
	rawMappings, _ := args["mappings"].([]any)
	if slug == "" || provider == "" || len(rawMappings) == 0 {
		return mcp.NewToolResultError("project_slug, provider y mappings (no vacío) requeridos"), nil
	}
	proj, perr := d.Projects.GetBySlug(ctx, orgID, slug)
	if perr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", slug)), nil
	}
	mappings := make([]ticketsvc.BulkLinkMapping, 0, len(rawMappings))
	for _, raw := range rawMappings {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		mp := ticketsvc.BulkLinkMapping{}
		if v, _ := m["ticket_id"].(string); v != "" {
			if id, err := uuid.Parse(v); err == nil {
				mp.TicketID = id
			}
		}
		if v, _ := m["ticket_key"].(string); v != "" {
			mp.TicketKey = v
		}
		if v, _ := m["external_id"].(string); v != "" {
			mp.ExternalID = v
		}
		if v, _ := m["external_url"].(string); v != "" {
			mp.ExternalURL = v
		}
		mappings = append(mappings, mp)
	}
	res, err := d.Tickets.BulkLinkExternal(ctx, orgID, proj.ID, provider, mappings)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("bulk link failed: %v", err)), nil
	}
	return toolResultJSON(res)
}

var _ context.Context
