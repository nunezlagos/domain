// Package mcpserver — HU-12.1 MCP server stdio.
//
// Define los tools nombrados con prefix `domain_*` que cualquier cliente MCP
// (Claude Code, otros agentes IA) puede invocar para persistir y buscar
// observations. Cada tool valida argumentos, llama al service correspondiente
// y formatea respuesta como mcp.CallToolResult.
//
// Principal:
//   El proceso domain-mcp resuelve UN principal al boot vía API key
//   (env var DOMAIN_API_KEY) y todas las tool calls de la sesión operan en
//   nombre de ese principal. Esto coincide con el modelo MCP stdio: un
//   proceso por sesión de cliente.
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"github.com/saargo/domain/internal/auth/apikey"
	agentsvc "github.com/saargo/domain/internal/service/agent"
	knowsvc "github.com/saargo/domain/internal/service/knowledge"
	obssvc "github.com/saargo/domain/internal/service/observation"
	projsvc "github.com/saargo/domain/internal/service/project"
	promptsvc "github.com/saargo/domain/internal/service/prompt"
	searchsvc "github.com/saargo/domain/internal/service/search"
	sesssvc "github.com/saargo/domain/internal/service/session"
	skillsvc "github.com/saargo/domain/internal/service/skill"
	timelinesvc "github.com/saargo/domain/internal/service/timeline"
)

// Deps colecciona las dependencias del servidor MCP.
type Deps struct {
	Observations *obssvc.Service
	Projects     *projsvc.Service
	Sessions     *sesssvc.Service
	Prompts      *promptsvc.Service
	Timeline     *timelinesvc.Service
	Search       *searchsvc.Service
	Knowledge    *knowsvc.Service
	Skills       *skillsvc.Service
	Agents       *agentsvc.Service
	Principal    *apikey.Principal // resuelto al boot
	ServerName   string
	ServerVer    string
}

// Tools construye la lista de mcpgo.ServerTool del proyecto (todos prefijo
// domain_*). Útil para tests in-process que reciben []ServerTool en
// mcptest.NewServer. Producción usa New() que internamente reusa Tools().
func Tools(deps Deps) []mcpgo.ServerTool {
	return []mcpgo.ServerTool{
		{Tool: toolMemSave(), Handler: deps.handleMemSave},
		{Tool: toolMemSearch(), Handler: deps.handleMemSearch},
		{Tool: toolMemContext(), Handler: deps.handleMemContext},
		{Tool: toolMemGetObservation(), Handler: deps.handleMemGetObservation},
		{Tool: toolSessionStart(), Handler: deps.handleSessionStart},
		{Tool: toolSessionEnd(), Handler: deps.handleSessionEnd},
		{Tool: toolSessionActive(), Handler: deps.handleSessionActive},
		{Tool: toolPromptGet(), Handler: deps.handlePromptGet},
		{Tool: toolPromptSearch(), Handler: deps.handlePromptSearch},
		{Tool: toolContext(), Handler: deps.handleContext},
		{Tool: toolTimeline(), Handler: deps.handleTimeline},
		{Tool: toolGlobalSearch(), Handler: deps.handleGlobalSearch},
		{Tool: toolKnowledgeSave(), Handler: deps.handleKnowledgeSave},
		{Tool: toolKnowledgeSearch(), Handler: deps.handleKnowledgeSearch},
		{Tool: toolKnowledgeGet(), Handler: deps.handleKnowledgeGet},
		{Tool: toolSkillList(), Handler: deps.handleSkillList},
		{Tool: toolSkillSearch(), Handler: deps.handleSkillSearch},
		{Tool: toolSkillGet(), Handler: deps.handleSkillGet},
		{Tool: toolAgentList(), Handler: deps.handleAgentList},
		{Tool: toolAgentGet(), Handler: deps.handleAgentGet},
	}
}

// New monta el servidor MCP con los tools del prefijo `domain_*`.
func New(deps Deps) *mcpgo.MCPServer {
	srv := mcpgo.NewMCPServer(
		deps.ServerName,
		deps.ServerVer,
		mcpgo.WithToolCapabilities(true),
	)
	srv.AddTools(Tools(deps)...)
	return srv
}

// --- tool builders (separados para reuso entre New y Tools list) ---

func toolMemSave() mcp.Tool {
	return mcp.NewTool("domain_mem_save",
		mcp.WithDescription("Guarda una observación de memoria en el project indicado. Genera embedding automáticamente para búsqueda híbrida."),
		mcp.WithString("project_slug",
			mcp.Description("Slug del project donde guardar"),
			mcp.Required(),
		),
		mcp.WithString("content",
			mcp.Description("Contenido de la observación (texto libre)"),
			mcp.Required(),
		),
		mcp.WithString("observation_type",
			mcp.Description("Tipo: note | decision | bug | pattern | discovery (default: note)"),
		),
		mcp.WithArray("tags",
			mcp.Description("Tags opcionales para categorizar"),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithObject("metadata",
			mcp.Description("Metadata estructurada arbitraria (JSONB)"),
		),
	)
}

func toolMemSearch() mcp.Tool {
	return mcp.NewTool("domain_mem_search",
		mcp.WithDescription("Busca observations relevantes a una query usando búsqueda híbrida BM25 + cosine + RRF fusion."),
		mcp.WithString("query",
			mcp.Description("Texto a buscar"),
			mcp.Required(),
		),
		mcp.WithNumber("limit",
			mcp.Description("Máximo resultados (default 20, max 100)"),
		),
	)
}

func toolMemContext() mcp.Tool {
	return mcp.NewTool("domain_mem_context",
		mcp.WithDescription("Recupera las últimas N observations de un project, ordenadas por fecha desc. Útil para contexto de sesión."),
		mcp.WithString("project_slug",
			mcp.Description("Slug del project"),
			mcp.Required(),
		),
		mcp.WithNumber("limit",
			mcp.Description("Máximo resultados (default 20, max 200)"),
		),
	)
}

func toolSessionStart() mcp.Tool {
	return mcp.NewTool("domain_session_start",
		mcp.WithDescription("Inicia una nueva session (agrupador de observations) opcionalmente scoped a un project."),
		mcp.WithString("title",
			mcp.Description("Título descriptivo de la sesión"),
			mcp.Required(),
		),
		mcp.WithString("project_slug",
			mcp.Description("Slug del project (opcional)"),
		),
		mcp.WithArray("tags",
			mcp.Description("Tags opcionales"),
			mcp.Items(map[string]any{"type": "string"}),
		),
	)
}

func toolSessionEnd() mcp.Tool {
	return mcp.NewTool("domain_session_end",
		mcp.WithDescription("Finaliza una session activa con un resumen opcional."),
		mcp.WithString("session_id",
			mcp.Description("UUID de la session"),
			mcp.Required(),
		),
		mcp.WithString("summary",
			mcp.Description("Resumen de lo realizado en la sesión"),
		),
	)
}

func toolSessionActive() mcp.Tool {
	return mcp.NewTool("domain_session_active",
		mcp.WithDescription("Devuelve la session activa del user actual (opcional: filtrar por project)."),
		mcp.WithString("project_slug",
			mcp.Description("Filtrar por project (opcional)"),
		),
	)
}

func toolPromptGet() mcp.Tool {
	return mcp.NewTool("domain_prompt_get",
		mcp.WithDescription("Obtiene la versión ACTIVA de un prompt template por slug. Útil para inyectar prompts en runs."),
		mcp.WithString("slug",
			mcp.Description("Slug del prompt template"),
			mcp.Required(),
		),
		mcp.WithString("project_slug",
			mcp.Description("Slug del project (opcional; si vacío usa prompts globales de la org)"),
		),
	)
}

func toolPromptSearch() mcp.Tool {
	return mcp.NewTool("domain_prompt_search",
		mcp.WithDescription("Busca prompts por contenido (full-text en español) con headline destacado."),
		mcp.WithString("query",
			mcp.Description("Texto a buscar en el body del prompt"),
			mcp.Required(),
		),
		mcp.WithNumber("limit",
			mcp.Description("Máximo resultados (default 20)"),
		),
	)
}

func toolContext() mcp.Tool {
	return mcp.NewTool("domain_context_snapshot",
		mcp.WithDescription("Devuelve snapshot del contexto: active_session + recientes (sessions, observations, prompts) para un project."),
		mcp.WithString("project_slug",
			mcp.Description("Slug del project (opcional; vacío = scope org-wide)"),
		),
	)
}

func toolTimeline() mcp.Tool {
	return mcp.NewTool("domain_timeline",
		mcp.WithDescription("Vecindario cronológico de una observation: N entradas antes y después incluyendo observations + prompts del mismo project."),
		mcp.WithString("observation_id",
			mcp.Description("UUID de la observation ancla"),
			mcp.Required(),
		),
		mcp.WithNumber("before",
			mcp.Description("Entradas anteriores (default 3, max 50)"),
		),
		mcp.WithNumber("after",
			mcp.Description("Entradas posteriores (default 3, max 50)"),
		),
	)
}

func toolGlobalSearch() mcp.Tool {
	return mcp.NewTool("domain_search_global",
		mcp.WithDescription("Búsqueda global cross-entity (observations + prompts + sessions) scoped por org del principal. Filtros opcionales."),
		mcp.WithString("query",
			mcp.Description("Texto a buscar"),
			mcp.Required(),
		),
		mcp.WithNumber("limit",
			mcp.Description("Máximo resultados (default 20, max 200)"),
		),
		mcp.WithArray("entity_types",
			mcp.Description("Filtrar a tipos específicos: observation, prompt, session"),
			mcp.Items(map[string]any{"type": "string"}),
		),
		mcp.WithArray("tags",
			mcp.Description("Tags requeridos (AND)"),
			mcp.Items(map[string]any{"type": "string"}),
		),
	)
}

func toolAgentList() mcp.Tool {
	return mcp.NewTool("domain_agent_list",
		mcp.WithDescription("Lista los agents disponibles en la org."),
		mcp.WithNumber("limit",
			mcp.Description("Máximo resultados (default 50)"),
		),
	)
}

func toolAgentGet() mcp.Tool {
	return mcp.NewTool("domain_agent_get",
		mcp.WithDescription("Recupera detalle de un agent por id o slug."),
		mcp.WithString("id",
			mcp.Description("UUID del agent (opcional si se pasa slug)"),
		),
		mcp.WithString("slug",
			mcp.Description("Slug del agent (opcional si se pasa id)"),
		),
	)
}

func toolSkillList() mcp.Tool {
	return mcp.NewTool("domain_skill_list",
		mcp.WithDescription("Lista los skills disponibles en la org. Filtros opcionales por type/tag."),
		mcp.WithString("type",
			mcp.Description("Filtrar por tipo: prompt | code | api | mcp_tool"),
		),
		mcp.WithString("tag",
			mcp.Description("Filtrar por tag específico"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Máximo resultados (default 50)"),
		),
	)
}

func toolSkillSearch() mcp.Tool {
	return mcp.NewTool("domain_skill_search",
		mcp.WithDescription("Busca skills por similitud semántica + BM25 sobre name+description."),
		mcp.WithString("query",
			mcp.Description("Texto descriptivo del capability buscado"),
			mcp.Required(),
		),
		mcp.WithNumber("limit",
			mcp.Description("Máximo resultados (default 20)"),
		),
	)
}

func toolSkillGet() mcp.Tool {
	return mcp.NewTool("domain_skill_get",
		mcp.WithDescription("Recupera detalle completo de un skill por id o slug."),
		mcp.WithString("id",
			mcp.Description("UUID del skill (opcional si se pasa slug)"),
		),
		mcp.WithString("slug",
			mcp.Description("Slug del skill (opcional si se pasa id)"),
		),
	)
}

func toolKnowledgeSave() mcp.Tool {
	return mcp.NewTool("domain_knowledge_save",
		mcp.WithDescription("Guarda un documento de conocimiento. Se chunkea automáticamente y genera embeddings por chunk para RAG."),
		mcp.WithString("project_slug",
			mcp.Description("Slug del project donde guardar"),
			mcp.Required(),
		),
		mcp.WithString("title",
			mcp.Description("Título del documento"),
			mcp.Required(),
		),
		mcp.WithString("body",
			mcp.Description("Contenido completo (texto largo OK; se chunkea)"),
			mcp.Required(),
		),
		mcp.WithString("source",
			mcp.Description("Fuente: manual | imported | web | etc. (default manual)"),
		),
		mcp.WithString("source_url",
			mcp.Description("URL origen si aplica"),
		),
		mcp.WithArray("tags",
			mcp.Description("Tags opcionales"),
			mcp.Items(map[string]any{"type": "string"}),
		),
	)
}

func toolKnowledgeSearch() mcp.Tool {
	return mcp.NewTool("domain_knowledge_search",
		mcp.WithDescription("Búsqueda híbrida (vector + BM25 + RRF) sobre chunks de knowledge documents."),
		mcp.WithString("query",
			mcp.Description("Texto de búsqueda"),
			mcp.Required(),
		),
		mcp.WithNumber("limit",
			mcp.Description("Máximo resultados (default 20)"),
		),
	)
}

func toolKnowledgeGet() mcp.Tool {
	return mcp.NewTool("domain_knowledge_get",
		mcp.WithDescription("Recupera un knowledge document completo (todos sus chunks reconstruidos)."),
		mcp.WithString("id",
			mcp.Description("UUID del documento"),
			mcp.Required(),
		),
	)
}

func toolMemGetObservation() mcp.Tool {
	return mcp.NewTool("domain_mem_get_observation",
		mcp.WithDescription("Recupera una observation específica por ID (UUID)."),
		mcp.WithString("id",
			mcp.Description("UUID de la observation"),
			mcp.Required(),
		),
	)
}

// --- handlers ---

func (d *Deps) handleMemSave(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	args := req.GetArguments()

	projectSlug, _ := args["project_slug"].(string)
	content, _ := args["content"].(string)
	obsType, _ := args["observation_type"].(string)

	if projectSlug == "" || content == "" {
		return mcp.NewToolResultError("project_slug y content son requeridos"), nil
	}

	orgID, err := uuid.Parse(d.Principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	userID, _ := uuid.Parse(d.Principal.UserID)

	proj, err := d.Projects.GetBySlug(ctx, orgID, projectSlug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found: %v", projectSlug, err)), nil
	}

	var tags []string
	if v, ok := args["tags"].([]any); ok {
		for _, t := range v {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
	}
	var metadata map[string]any
	if v, ok := args["metadata"].(map[string]any); ok {
		metadata = v
	}

	obs, err := d.Observations.Save(ctx, obssvc.SaveInput{
		OrganizationID:  orgID,
		ProjectID:       proj.ID,
		CreatedBy:       &userID,
		Content:         content,
		ObservationType: obsType,
		Tags:            tags,
		Metadata:        metadata,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("save failed: %v", err)), nil
	}

	return toolResultJSON(map[string]any{
		"id":         obs.ID.String(),
		"project_id": obs.ProjectID.String(),
		"created_at": obs.CreatedAt,
		"message":    "observation saved",
	})
}

func (d *Deps) handleMemSearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	args := req.GetArguments()
	query, _ := args["query"].(string)
	if query == "" {
		return mcp.NewToolResultError("query requerido"), nil
	}
	limit := 20
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	results, err := d.Observations.SearchHybrid(ctx, orgID, query, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}
	out := make([]map[string]any, 0, len(results))
	for _, r := range results {
		out = append(out, map[string]any{
			"id":               r.ID.String(),
			"content":          r.Content,
			"observation_type": r.ObservationType,
			"tags":             r.Tags,
			"score":            r.Score,
			"bm25_rank":        r.BM25Rank,
			"vector_rank":      r.VectorRank,
			"created_at":       r.CreatedAt,
		})
	}
	return toolResultJSON(map[string]any{
		"results": out,
		"count":   len(out),
	})
}

func (d *Deps) handleMemContext(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	args := req.GetArguments()
	projectSlug, _ := args["project_slug"].(string)
	if projectSlug == "" {
		return mcp.NewToolResultError("project_slug requerido"), nil
	}
	limit := 20
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	proj, err := d.Projects.GetBySlug(ctx, orgID, projectSlug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project not found: %v", err)), nil
	}
	obs, err := d.Observations.List(ctx, proj.ID, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list failed: %v", err)), nil
	}
	out := make([]map[string]any, 0, len(obs))
	for _, o := range obs {
		out = append(out, map[string]any{
			"id":               o.ID.String(),
			"content":          o.Content,
			"observation_type": o.ObservationType,
			"tags":             o.Tags,
			"created_at":       o.CreatedAt,
		})
	}
	return toolResultJSON(map[string]any{
		"project_slug": projectSlug,
		"results":      out,
		"count":        len(out),
	})
}

func (d *Deps) handleSessionStart(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Sessions == nil {
		return mcp.NewToolResultError("session service no configurado"), nil
	}
	args := req.GetArguments()
	title, _ := args["title"].(string)
	if title == "" {
		return mcp.NewToolResultError("title requerido"), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	userID, _ := uuid.Parse(d.Principal.UserID)

	var projectID *uuid.UUID
	if slug, _ := args["project_slug"].(string); slug != "" {
		proj, err := d.Projects.GetBySlug(ctx, orgID, slug)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", slug)), nil
		}
		projectID = &proj.ID
	}
	var tags []string
	if v, ok := args["tags"].([]any); ok {
		for _, t := range v {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
	}
	sess, err := d.Sessions.Start(ctx, sesssvc.StartInput{
		OrganizationID: orgID, UserID: userID, ProjectID: projectID, Title: title, Tags: tags,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("start: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"id":         sess.ID.String(),
		"started_at": sess.StartedAt,
		"status":     sess.Status(),
	})
}

func (d *Deps) handleSessionEnd(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Sessions == nil {
		return mcp.NewToolResultError("session service no configurado"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["session_id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("session_id inválido (UUID requerido)"), nil
	}
	userID, _ := uuid.Parse(d.Principal.UserID)
	summary, _ := args["summary"].(string)
	sess, err := d.Sessions.End(ctx, id, userID, summary)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("end: %v", err)), nil
	}
	if sess.OrganizationID.String() != d.Principal.OrganizationID {
		return mcp.NewToolResultError("not found"), nil
	}
	return toolResultJSON(sess)
}

func (d *Deps) handleSessionActive(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Sessions == nil {
		return mcp.NewToolResultError("session service no configurado"), nil
	}
	args := req.GetArguments()
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	userID, _ := uuid.Parse(d.Principal.UserID)

	var projectID uuid.UUID
	if slug, _ := args["project_slug"].(string); slug != "" {
		proj, err := d.Projects.GetBySlug(ctx, orgID, slug)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", slug)), nil
		}
		projectID = proj.ID
	}
	sess, err := d.Sessions.GetActive(ctx, userID, projectID)
	if err != nil {
		// No hay sesión activa: devolvemos null en lugar de error
		return toolResultJSON(nil)
	}
	return toolResultJSON(sess)
}

func (d *Deps) handlePromptGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Prompts == nil {
		return mcp.NewToolResultError("prompt service no configurado"), nil
	}
	args := req.GetArguments()
	slug, _ := args["slug"].(string)
	if slug == "" {
		return mcp.NewToolResultError("slug requerido"), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	var projectID *uuid.UUID
	if ps, _ := args["project_slug"].(string); ps != "" {
		proj, err := d.Projects.GetBySlug(ctx, orgID, ps)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", ps)), nil
		}
		projectID = &proj.ID
	}
	p, err := d.Prompts.GetActive(ctx, orgID, projectID, slug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get_active: %v", err)), nil
	}
	return toolResultJSON(p)
}

func (d *Deps) handlePromptSearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Prompts == nil {
		return mcp.NewToolResultError("prompt service no configurado"), nil
	}
	args := req.GetArguments()
	query, _ := args["query"].(string)
	if query == "" {
		return mcp.NewToolResultError("query requerido"), nil
	}
	limit := 20
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	results, err := d.Prompts.Search(ctx, orgID, query, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"results": results,
		"count":   len(results),
	})
}

func (d *Deps) handleContext(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Timeline == nil {
		return mcp.NewToolResultError("timeline service no configurado"), nil
	}
	args := req.GetArguments()
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	userID, _ := uuid.Parse(d.Principal.UserID)
	var projectID uuid.UUID
	if ps, _ := args["project_slug"].(string); ps != "" {
		proj, err := d.Projects.GetBySlug(ctx, orgID, ps)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", ps)), nil
		}
		projectID = proj.ID
	}
	snap, err := d.Timeline.Context(ctx, orgID, userID, projectID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("context: %v", err)), nil
	}
	return toolResultJSON(snap)
}

func (d *Deps) handleTimeline(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Timeline == nil {
		return mcp.NewToolResultError("timeline service no configurado"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["observation_id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("observation_id inválido"), nil
	}
	before := 3
	after := 3
	if v, ok := args["before"].(float64); ok {
		before = int(v)
	}
	if v, ok := args["after"].(float64); ok {
		after = int(v)
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	entries, err := d.Timeline.Timeline(ctx, orgID, id, before, after)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("timeline: %v", err)), nil
	}
	return toolResultJSON(entries)
}

func (d *Deps) handleGlobalSearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Search == nil {
		return mcp.NewToolResultError("search service no configurado"), nil
	}
	args := req.GetArguments()
	query, _ := args["query"].(string)
	if query == "" {
		return mcp.NewToolResultError("query requerido"), nil
	}
	limit := 20
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}
	filter := searchsvc.Filter{}
	if et, ok := args["entity_types"].([]any); ok {
		for _, t := range et {
			if s, ok := t.(string); ok {
				filter.EntityTypes = append(filter.EntityTypes, searchsvc.EntityType(s))
			}
		}
	}
	if tg, ok := args["tags"].([]any); ok {
		for _, t := range tg {
			if s, ok := t.(string); ok {
				filter.Tags = append(filter.Tags, s)
			}
		}
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	results, err := d.Search.Search(ctx, orgID, query, limit, filter)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"results": results,
		"count":   len(results),
	})
}

func (d *Deps) handleAgentList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Agents == nil {
		return mcp.NewToolResultError("agent service no configurado"), nil
	}
	args := req.GetArguments()
	limit := 50
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	out, err := d.Agents.List(ctx, orgID, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"results": out, "count": len(out)})
}

func (d *Deps) handleAgentGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Agents == nil {
		return mcp.NewToolResultError("agent service no configurado"), nil
	}
	args := req.GetArguments()
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	if idStr, _ := args["id"].(string); idStr != "" {
		id, err := uuid.Parse(idStr)
		if err != nil {
			return mcp.NewToolResultError("id inválido"), nil
		}
		ag, err := d.Agents.GetByID(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get: %v", err)), nil
		}
		if ag.OrganizationID != orgID {
			return mcp.NewToolResultError("not found"), nil
		}
		return toolResultJSON(ag)
	}
	slug, _ := args["slug"].(string)
	if slug == "" {
		return mcp.NewToolResultError("id o slug requerido"), nil
	}
	ag, err := d.Agents.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get: %v", err)), nil
	}
	return toolResultJSON(ag)
}

func (d *Deps) handleSkillList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Skills == nil {
		return mcp.NewToolResultError("skill service no configurado"), nil
	}
	args := req.GetArguments()
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	f := skillsvc.ListFilter{}
	if v, _ := args["type"].(string); v != "" {
		f.SkillType = v
	}
	if v, _ := args["tag"].(string); v != "" {
		f.Tag = v
	}
	if v, ok := args["limit"].(float64); ok {
		f.Limit = int(v)
	}
	out, err := d.Skills.List(ctx, orgID, f)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"results": out, "count": len(out)})
}

func (d *Deps) handleSkillSearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Skills == nil {
		return mcp.NewToolResultError("skill service no configurado"), nil
	}
	args := req.GetArguments()
	query, _ := args["query"].(string)
	if query == "" {
		return mcp.NewToolResultError("query requerido"), nil
	}
	limit := 20
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	results, err := d.Skills.SearchHybrid(ctx, orgID, query, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"results": results, "count": len(results)})
}

func (d *Deps) handleSkillGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Skills == nil {
		return mcp.NewToolResultError("skill service no configurado"), nil
	}
	args := req.GetArguments()
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	if idStr, _ := args["id"].(string); idStr != "" {
		id, err := uuid.Parse(idStr)
		if err != nil {
			return mcp.NewToolResultError("id inválido"), nil
		}
		sk, err := d.Skills.GetByID(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("get: %v", err)), nil
		}
		if sk.OrganizationID != orgID {
			return mcp.NewToolResultError("not found"), nil
		}
		return toolResultJSON(sk)
	}
	slug, _ := args["slug"].(string)
	if slug == "" {
		return mcp.NewToolResultError("id o slug requerido"), nil
	}
	sk, err := d.Skills.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get: %v", err)), nil
	}
	return toolResultJSON(sk)
}

func (d *Deps) handleKnowledgeSave(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Knowledge == nil {
		return mcp.NewToolResultError("knowledge service no configurado"), nil
	}
	args := req.GetArguments()
	slug, _ := args["project_slug"].(string)
	title, _ := args["title"].(string)
	body, _ := args["body"].(string)
	if slug == "" || title == "" || body == "" {
		return mcp.NewToolResultError("project_slug, title y body son requeridos"), nil
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	userID, _ := uuid.Parse(d.Principal.UserID)
	proj, err := d.Projects.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", slug)), nil
	}
	var tags []string
	if v, ok := args["tags"].([]any); ok {
		for _, t := range v {
			if s, ok := t.(string); ok {
				tags = append(tags, s)
			}
		}
	}
	source, _ := args["source"].(string)
	sourceURL, _ := args["source_url"].(string)
	doc, chunks, err := d.Knowledge.Save(ctx, knowsvc.SaveInput{
		OrganizationID: orgID, ProjectID: proj.ID, CreatedBy: &userID,
		Title: title, Body: body, Source: source, SourceURL: sourceURL, Tags: tags,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("save: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"id":           doc.ID.String(),
		"chunks_count": len(chunks),
		"created_at":   doc.CreatedAt,
	})
}

func (d *Deps) handleKnowledgeSearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Knowledge == nil {
		return mcp.NewToolResultError("knowledge service no configurado"), nil
	}
	args := req.GetArguments()
	query, _ := args["query"].(string)
	if query == "" {
		return mcp.NewToolResultError("query requerido"), nil
	}
	limit := 20
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	results, err := d.Knowledge.SearchHybrid(ctx, orgID, query, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"results": results,
		"count":   len(results),
	})
}

func (d *Deps) handleKnowledgeGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Knowledge == nil {
		return mcp.NewToolResultError("knowledge service no configurado"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id inválido (UUID)"), nil
	}
	doc, chunks, err := d.Knowledge.Get(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get: %v", err)), nil
	}
	if doc.OrganizationID.String() != d.Principal.OrganizationID {
		return mcp.NewToolResultError("not found"), nil
	}
	return toolResultJSON(map[string]any{
		"document": doc,
		"chunks":   chunks,
	})
}

func (d *Deps) handleMemGetObservation(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("invalid id (UUID expected)"), nil
	}
	obs, err := d.Observations.Get(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get failed: %v", err)), nil
	}
	// Cross-org leak guard
	if obs.OrganizationID.String() != d.Principal.OrganizationID {
		return mcp.NewToolResultError("not found"), nil
	}
	return toolResultJSON(obs)
}

func toolResultJSON(v any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal: %v", err)), nil
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{mcp.TextContent{Type: "text", Text: string(b)}},
	}, nil
}
