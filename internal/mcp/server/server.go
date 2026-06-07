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
	obssvc "github.com/saargo/domain/internal/service/observation"
	projsvc "github.com/saargo/domain/internal/service/project"
)

// Deps colecciona las dependencias del servidor MCP.
type Deps struct {
	Observations *obssvc.Service
	Projects     *projsvc.Service
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
