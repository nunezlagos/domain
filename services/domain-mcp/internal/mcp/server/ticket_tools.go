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

	"nunezlagos/domain/internal/auth/apikey"
	clientsvc "nunezlagos/domain/internal/service/client"
	projsvc "nunezlagos/domain/internal/service/project"
	ticketsvc "nunezlagos/domain/internal/service/ticket"
)

type ticketService interface {
	Create(ctx context.Context, in ticketsvc.CreateInput) (*ticketsvc.Ticket, error)
	Get(ctx context.Context, orgID, id uuid.UUID) (*ticketsvc.Ticket, error)
	GetByKey(ctx context.Context, orgID, projectID uuid.UUID, key string) (*ticketsvc.Ticket, error)
	List(ctx context.Context, orgID uuid.UUID, filter ticketsvc.ListFilter) ([]*ticketsvc.Ticket, int64, error)
	UpdateAs(ctx context.Context, orgID, id, actor uuid.UUID, in ticketsvc.UpdateInput) (*ticketsvc.Ticket, error)
	ChangeStatus(ctx context.Context, orgID, id uuid.UUID, toStatus string, actorID uuid.UUID, note string) (*ticketsvc.Ticket, error)
	Delete(ctx context.Context, orgID, id uuid.UUID) error
	AddComment(ctx context.Context, ticketID, authorID uuid.UUID, body string) (*ticketsvc.Comment, error)
	ListComments(ctx context.Context, ticketID uuid.UUID) ([]*ticketsvc.Comment, error)
	StatusHistory(ctx context.Context, ticketID uuid.UUID) ([]*ticketsvc.StatusChange, error)
	LinkExternal(ctx context.Context, orgID, id uuid.UUID, link ticketsvc.ExternalLink) (*ticketsvc.Ticket, error)
	UnlinkExternal(ctx context.Context, orgID, id uuid.UUID) error
	LinkIssue(ctx context.Context, orgID, ticketID uuid.UUID, issueID *uuid.UUID) (*ticketsvc.Ticket, error)
	BulkLinkExternal(ctx context.Context, orgID, projectID uuid.UUID, provider string, mappings []ticketsvc.BulkLinkMapping) (*ticketsvc.BulkLinkResult, error)
	Claim(ctx context.Context, orgID, ticketID, userID uuid.UUID, ttlMinutes int) (*ticketsvc.Ticket, error)
	Release(ctx context.Context, orgID, ticketID, userID uuid.UUID) (*ticketsvc.Ticket, error)
	Reassign(ctx context.Context, orgID, ticketID uuid.UUID, newAssignee *uuid.UUID) (*ticketsvc.Ticket, error)
}

type ticketProjectGetter interface {
	GetBySlug(ctx context.Context, orgID uuid.UUID, slug string) (*projsvc.Project, error)
}

type optionalClientService interface {
	Get(ctx context.Context, orgID uuid.UUID, idOrSlug string) (*clientsvc.Client, error)
}

type ticketHandlers struct {
	tickets   ticketService
	projects  ticketProjectGetter
	clients   optionalClientService
	principal *apikey.Principal
}

func registerTicketTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	h := &ticketHandlers{
		tickets:   deps.Tickets,
		projects:  deps.Projects,
		clients:   deps.Clients,
		principal: deps.Principal,
	}
	rls := func(fn mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withOrgTxHandler(&deps, fn)
	}
	return []mcpgo.ServerTool{
		{Tool: toolTicketCreate(), Handler: wrap.Wrap("domain_ticket_create", rls(h.handleTicketCreate))},
		{Tool: toolTicketGet(), Handler: wrap.Wrap("domain_ticket_get", rls(h.handleTicketGet))},
		{Tool: toolTicketList(), Handler: wrap.Wrap("domain_ticket_list", rls(h.handleTicketList))},
		{Tool: toolTicketUpdate(), Handler: wrap.Wrap("domain_ticket_update", rls(h.handleTicketUpdate))},
		{Tool: toolTicketChangeStatus(), Handler: wrap.Wrap("domain_ticket_change_status", rls(h.handleTicketChangeStatus))},
		{Tool: toolTicketDelete(), Handler: wrap.Wrap("domain_ticket_delete", rls(h.handleTicketDelete))},
		{Tool: toolTicketCommentAdd(), Handler: wrap.Wrap("domain_ticket_comment_add", rls(h.handleTicketCommentAdd))},
		{Tool: toolTicketCommentList(), Handler: wrap.Wrap("domain_ticket_comment_list", rls(h.handleTicketCommentList))},
		{Tool: toolTicketStatusHistory(), Handler: wrap.Wrap("domain_ticket_status_history", rls(h.handleTicketStatusHistory))},
		{Tool: toolTicketLinkExternal(), Handler: wrap.Wrap("domain_ticket_link_external", rls(h.handleTicketLinkExternal))},
		{Tool: toolTicketLinkIssue(), Handler: wrap.Wrap("domain_ticket_link_issue", rls(h.handleTicketLinkIssue))},
		{Tool: toolTicketLinkExternalBulk(), Handler: wrap.Wrap("domain_ticket_link_external_bulk", rls(h.handleTicketLinkExternalBulk))},
		{Tool: toolTicketClaim(), Handler: wrap.Wrap("domain_ticket_claim", rls(h.handleTicketClaim))},
		{Tool: toolTicketRelease(), Handler: wrap.Wrap("domain_ticket_release", rls(h.handleTicketRelease))},
		{Tool: toolTicketReassign(), Handler: wrap.Wrap("domain_ticket_reassign", rls(h.handleTicketReassign))},
	}
}

// REQ-63 tool definitions
func toolTicketClaim() mcp.Tool {
	return mcp.NewTool("domain_ticket_claim",
		mcp.WithDescription("Adquiere un soft-lock cooperativo sobre el ticket. Mientras lo tengas, otros users que intenten Update/ChangeStatus reciben 409 'lockeado por otro'. Self-renew es OK. El lock expira solo tras ttl_minutes (default 30, max 240). Si quiere tomarle el ticket a otro, llama domain_ticket_reassign (no Claim)."),
		mcp.WithString("id", mcp.Description("UUID del ticket"), mcp.Required()),
		mcp.WithNumber("ttl_minutes", mcp.Description("Tiempo de vida del lock en minutos. Default 30, max 240.")),
	)
}

func toolTicketRelease() mcp.Tool {
	return mcp.NewTool("domain_ticket_release",
		mcp.WithDescription("Suelta el lock que tiene sobre el ticket. Idempotente — si no habia lock, no-op. Si el lock es de otro y no expiro, falla con 'lockeado por otro'."),
		mcp.WithString("id", mcp.Description("UUID del ticket"), mcp.Required()),
	)
}

func toolTicketReassign() mcp.Tool {
	return mcp.NewTool("domain_ticket_reassign",
		mcp.WithDescription("Cambia el assignee del ticket bypaseando el lock (uso tipico: dashboard reasignando un ticket retenido). assignee_id vacio o '00000000-...' = des-asignar."),
		mcp.WithString("id", mcp.Description("UUID del ticket"), mcp.Required()),
		mcp.WithString("assignee_id", mcp.Description("UUID del nuevo assignee, o '' para des-asignar")),
	)
}

func (h *ticketHandlers) handleTicketClaim(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	userID, _ := uuid.Parse(h.principal.UserID)
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id invalido"), nil
	}
	ttl := 0
	if v, ok := args["ttl_minutes"].(float64); ok {
		ttl = int(v)
	}
	t, err := h.tickets.Claim(ctx, orgID, id, userID, ttl)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("claim failed: %v", err)), nil
	}
	return toolResultJSON(t)
}

func (h *ticketHandlers) handleTicketRelease(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	userID, _ := uuid.Parse(h.principal.UserID)
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id invalido"), nil
	}
	t, err := h.tickets.Release(ctx, orgID, id, userID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("release failed: %v", err)), nil
	}
	return toolResultJSON(t)
}

func (h *ticketHandlers) handleTicketReassign(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id invalido"), nil
	}
	var assignee *uuid.UUID
	if v, ok := args["assignee_id"].(string); ok && v != "" {
		if aid, err := uuid.Parse(v); err == nil && aid != uuid.Nil {
			assignee = &aid
		}
	}
	t, err := h.tickets.Reassign(ctx, orgID, id, assignee)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("reassign failed: %v", err)), nil
	}
	return toolResultJSON(t)
}

func toolTicketCreate() mcp.Tool {
	return mcp.NewTool("domain_ticket_create",
		mcp.WithDescription("Crea un ticket interno en un proyecto. Auto-genera key (PROJ-1, PROJ-2 derivado del project slug). issue_type: bug|feature|requirement|task|epic|improvement|spike. priority: trivial|low|medium|high|critical. status arranca en 'backlog'. Para vincular con Jira/etc usa domain_ticket_link_external despues."),
		mcp.WithString("project_slug", mcp.Description("Proyecto al que pertenece el ticket"), mcp.Required()),
		mcp.WithString("title", mcp.Description("Titulo corto"), mcp.Required()),
		mcp.WithString("description_md", mcp.Description("Descripcion markdown")),
		mcp.WithString("issue_type", mcp.Description("bug|feature|requirement|task|epic|improvement|spike. Default: task")),
		mcp.WithString("priority", mcp.Description("trivial|low|medium|high|critical. Default: medium")),
		mcp.WithString("client_slug", mcp.Description("Si el ticket pertenece a un cliente/mandante especifico del proyecto")),
		mcp.WithString("assignee_id", mcp.Description("UUID del usuario asignado (opcional)")),
		mcp.WithArray("labels", mcp.Description("Tags libres ej: ['urgente','frontend']")),
		mcp.WithString("parent_id", mcp.Description("UUID de epic/story padre (opcional)")),
		mcp.WithNumber("estimated_hours", mcp.Description("Estimacion en horas (decimal)")),
		mcp.WithString("due_date", mcp.Description("YYYY-MM-DD opcional")),
		mcp.WithString("external_provider", mcp.Description("REQ-58: si el ticket ya esta en Jira/etc, vincular en el mismo INSERT (jira|github|gitlab|linear|azure_devops).")),
		mcp.WithString("external_id", mcp.Description("Key externo (ej: MPS-12). Requiere external_provider.")),
		mcp.WithString("external_url", mcp.Description("URL al ticket externo. Opcional.")),
	)
}

func toolTicketGet() mcp.Tool {
	return mcp.NewTool("domain_ticket_get",
		mcp.WithDescription("Obtiene un ticket por id o por key (ej: ACMEWEB-15 + project_slug). Si pasas ambos, gana id."),
		mcp.WithString("id", mcp.Description("UUID del ticket")),
		mcp.WithString("project_slug", mcp.Description("Si vas a buscar por key")),
		mcp.WithString("key", mcp.Description("Ej: ACMEWEB-15")),
	)
}

func toolTicketList() mcp.Tool {
	return mcp.NewTool("domain_ticket_list",
		mcp.WithDescription("Lista tickets filtrados por proyecto/status/type/priority/assignee/label/parent/busqueda. Default ordena por updated_at DESC."),
		mcp.WithString("project_slug", mcp.Description("Filtrar por proyecto")),
		mcp.WithString("status", mcp.Description("backlog|todo|in_progress|in_review|blocked|done|cancelled")),
		mcp.WithString("issue_type", mcp.Description("bug|feature|...")),
		mcp.WithString("priority", mcp.Description("trivial|low|medium|high|critical")),
		mcp.WithString("assignee_id", mcp.Description("UUID del assignee")),
		mcp.WithString("reporter_id", mcp.Description("UUID del reporter")),
		mcp.WithString("parent_id", mcp.Description("UUID del epic/story padre — para listar subtasks")),
		mcp.WithString("label", mcp.Description("Filtrar por una label especifica")),
		mcp.WithString("query", mcp.Description("Full-text search sobre title+description")),
		mcp.WithNumber("limit", mcp.Description("Default 50, max 200")),
		mcp.WithNumber("offset", mcp.Description("Default 0")),
	)
}

func toolTicketUpdate() mcp.Tool {
	return mcp.NewTool("domain_ticket_update",
		mcp.WithDescription("Update parcial. Solo los campos provistos se modifican. Para cambiar status usa domain_ticket_change_status (registra history). Para des-asignar pasa assignee_id='00000000-0000-0000-0000-000000000000'."),
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
		mcp.WithDescription("Transicion de status. Registra entry en status_history con quien y por que. Auto-setea started_at en primer in_progress y completed_at en done/cancelled."),
		mcp.WithString("id", mcp.Description("UUID del ticket"), mcp.Required()),
		mcp.WithString("to_status", mcp.Description("backlog|todo|in_progress|in_review|blocked|done|cancelled"), mcp.Required()),
		mcp.WithString("note", mcp.Description("Razon / comentario de la transicion")),
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
		mcp.WithDescription("Lista comments del ticket en orden cronologico ASC."),
		mcp.WithString("ticket_id", mcp.Description("UUID"), mcp.Required()),
	)
}

func toolTicketStatusHistory() mcp.Tool {
	return mcp.NewTool("domain_ticket_status_history",
		mcp.WithDescription("Devuelve la historia de transiciones de status (audit). Util para calcular cycle time, lead time, ver quien bloqueo que."),
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

func (h *ticketHandlers) requireDeps() error {
	if h.principal == nil {
		return fmt.Errorf("no authenticated principal")
	}
	if h.tickets == nil {
		return fmt.Errorf("tickets service not configured")
	}
	if h.projects == nil {
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

func (h *ticketHandlers) handleTicketCreate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := req.GetArguments()
	projSlug, _ := args["project_slug"].(string)
	title, _ := args["title"].(string)
	if projSlug == "" || title == "" {
		return mcp.NewToolResultError("project_slug y title requeridos"), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	userID, _ := uuid.Parse(h.principal.UserID)
	proj, perr := h.projects.GetBySlug(ctx, orgID, projSlug)
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
	if v, ok := args["client_slug"].(string); ok && v != "" && h.clients != nil {
		if cl, _ := h.clients.Get(ctx, orgID, v); cl != nil {
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

	t, err := h.tickets.Create(ctx, in)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create ticket failed: %v", err)), nil
	}
	return toolResultJSON(t)
}

func (h *ticketHandlers) handleTicketGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	args := req.GetArguments()
	if idStr, _ := args["id"].(string); idStr != "" {
		id, err := uuid.Parse(idStr)
		if err != nil {
			return mcp.NewToolResultError("id invalido"), nil
		}
		t, err := h.tickets.Get(ctx, orgID, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("ticket not found: %v", err)), nil
		}
		return toolResultJSON(t)
	}
	projSlug, _ := args["project_slug"].(string)
	key, _ := args["key"].(string)
	if projSlug == "" || key == "" {
		return mcp.NewToolResultError("pasa id o (project_slug + key)"), nil
	}
	proj, perr := h.projects.GetBySlug(ctx, orgID, projSlug)
	if perr != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", projSlug)), nil
	}
	t, err := h.tickets.GetByKey(ctx, orgID, proj.ID, key)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("ticket %s not found", key)), nil
	}
	return toolResultJSON(t)
}

func (h *ticketHandlers) handleTicketList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	args := req.GetArguments()
	filter := ticketsvc.ListFilter{}
	if v, ok := args["project_slug"].(string); ok && v != "" {
		if proj, perr := h.projects.GetBySlug(ctx, orgID, v); perr == nil {
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
	list, total, err := h.tickets.List(ctx, orgID, filter)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"tickets": list, "total": total})
}

func (h *ticketHandlers) handleTicketUpdate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	userID, _ := uuid.Parse(h.principal.UserID)
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id invalido"), nil
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
	t, err := h.tickets.UpdateAs(ctx, orgID, id, userID, in)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("update failed: %v", err)), nil
	}
	return toolResultJSON(t)
}

func (h *ticketHandlers) handleTicketChangeStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	userID, _ := uuid.Parse(h.principal.UserID)
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id invalido"), nil
	}
	toStatus, _ := args["to_status"].(string)
	if toStatus == "" {
		return mcp.NewToolResultError("to_status requerido"), nil
	}
	note, _ := args["note"].(string)
	t, err := h.tickets.ChangeStatus(ctx, orgID, id, toStatus, userID, note)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("change_status failed: %v", err)), nil
	}
	return toolResultJSON(t)
}

func (h *ticketHandlers) handleTicketDelete(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id invalido"), nil
	}
	if err := h.tickets.Delete(ctx, orgID, id); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("delete failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"id": id.String(), "deleted": true})
}

func (h *ticketHandlers) handleTicketCommentAdd(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	userID, _ := uuid.Parse(h.principal.UserID)
	args := req.GetArguments()
	idStr, _ := args["ticket_id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("ticket_id invalido"), nil
	}
	body, _ := args["body_md"].(string)
	c, err := h.tickets.AddComment(ctx, id, userID, body)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("comment failed: %v", err)), nil
	}
	return toolResultJSON(c)
}

func (h *ticketHandlers) handleTicketCommentList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := req.GetArguments()
	idStr, _ := args["ticket_id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("ticket_id invalido"), nil
	}
	out, err := h.tickets.ListComments(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list comments failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"comments": out, "total": len(out)})
}

func (h *ticketHandlers) handleTicketStatusHistory(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := req.GetArguments()
	idStr, _ := args["ticket_id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("ticket_id invalido"), nil
	}
	hist, err := h.tickets.StatusHistory(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("history failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"history": hist, "total": len(hist)})
}

func (h *ticketHandlers) handleTicketLinkExternal(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id invalido"), nil
	}
	provider, _ := args["provider"].(string)
	if provider == "" {
		if err := h.tickets.UnlinkExternal(ctx, orgID, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("unlink failed: %v", err)), nil
		}
		return toolResultJSON(map[string]any{"id": id.String(), "unlinked": true})
	}
	extID, _ := args["external_id"].(string)
	extURL, _ := args["external_url"].(string)
	t, err := h.tickets.LinkExternal(ctx, orgID, id, ticketsvc.ExternalLink{
		Provider: provider, ID: extID, URL: extURL,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("link failed: %v", err)), nil
	}
	return toolResultJSON(t)
}

func toolTicketLinkIssue() mcp.Tool {
	return mcp.NewTool("domain_ticket_link_issue",
		mcp.WithDescription("Vincula un ticket operativo con una HU/issue del workflow SDD (REQ-56). Pasar issue_id='' para desvincular. Util cuando el ticket implementa una HU formal con Gherkin scenarios."),
		mcp.WithString("ticket_id", mcp.Description("UUID del ticket"), mcp.Required()),
		mcp.WithString("issue_id", mcp.Description("UUID del issue (issues.id) o vacio para desvincular")),
	)
}

func (h *ticketHandlers) handleTicketLinkIssue(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	args := req.GetArguments()
	tStr, _ := args["ticket_id"].(string)
	tID, err := uuid.Parse(tStr)
	if err != nil {
		return mcp.NewToolResultError("ticket_id invalido"), nil
	}
	var issuePtr *uuid.UUID
	if iStr, _ := args["issue_id"].(string); iStr != "" {
		iID, perr := uuid.Parse(iStr)
		if perr != nil {
			return mcp.NewToolResultError("issue_id invalido"), nil
		}
		issuePtr = &iID
	}
	t, err := h.tickets.LinkIssue(ctx, orgID, tID, issuePtr)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("link_issue failed: %v", err)), nil
	}
	return toolResultJSON(t)
}

// REQ-58: bulk link (sync inicial Jira→domain)
func toolTicketLinkExternalBulk() mcp.Tool {
	return mcp.NewTool("domain_ticket_link_external_bulk",
		mcp.WithDescription("Vincula N tickets a sus externals en una operacion. Para sync inicial cuando se enchufa Jira/GitHub/etc y hay que linkear tickets existentes en bulk. provider: jira|github|gitlab|linear|azure_devops. mappings: array de {ticket_key|ticket_id, external_id, external_url}."),
		mcp.WithString("project_slug", mcp.Description("Proyecto donde estan los tickets"), mcp.Required()),
		mcp.WithString("provider", mcp.Description("Proveedor externo"), mcp.Required()),
		mcp.WithArray("mappings", mcp.Description("Array de mappings. Cada item: {ticket_key:'ACMEWEB-1', external_id:'MPS-12', external_url:'https://...'} o {ticket_id:UUID, ...}"), mcp.Required()),
	)
}

func (h *ticketHandlers) handleTicketLinkExternalBulk(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	args := req.GetArguments()
	slug, _ := args["project_slug"].(string)
	provider, _ := args["provider"].(string)
	rawMappings, _ := args["mappings"].([]any)
	if slug == "" || provider == "" || len(rawMappings) == 0 {
		return mcp.NewToolResultError("project_slug, provider y mappings (no vacio) requeridos"), nil
	}
	proj, perr := h.projects.GetBySlug(ctx, orgID, slug)
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
	res, err := h.tickets.BulkLinkExternal(ctx, orgID, proj.ID, provider, mappings)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("bulk link failed: %v", err)), nil
	}
	return toolResultJSON(res)
}

var _ context.Context
