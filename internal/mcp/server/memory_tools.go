// issue-12.2 — tools MCP de memoria faltantes: delete, save_prompt,
// capture_passive, suggest_topic_key y stats.
package mcpserver

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	obssvc "nunezlagos/domain/internal/service/observation"
	promptsvc "nunezlagos/domain/internal/service/prompt"
)

func registerMemoryTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	return []mcpgo.ServerTool{
		{Tool: toolMemDelete(), Handler: wrap.Wrap("domain_mem_delete", deps.handleMemDelete)},
		{Tool: toolMemSavePrompt(), Handler: wrap.Wrap("domain_mem_save_prompt", deps.handleMemSavePrompt)},
		{Tool: toolMemCapturePassive(), Handler: wrap.Wrap("domain_mem_capture_passive", deps.handleMemCapturePassive)},
		{Tool: toolMemSuggestTopicKey(), Handler: wrap.Wrap("domain_mem_suggest_topic_key", deps.handleMemSuggestTopicKey)},
		{Tool: toolMemStats(), Handler: wrap.Wrap("domain_mem_stats", deps.handleMemStats)},
	}
}

func toolMemDelete() mcp.Tool {
	return mcp.NewTool("domain_mem_delete",
		mcp.WithDescription("Elimina (soft-delete) una observación de memoria por id."),
		mcp.WithString("observation_id",
			mcp.Description("UUID de la observación a eliminar"),
			mcp.Required(),
		),
	)
}

func (d *Deps) handleMemDelete(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	// issue-25.14: wireup tx con SET LOCAL para RLS
	ctx, tx, release := withOrgCtx(ctx, d.Pool, d.Principal)
	defer release()
	args := req.GetArguments()
	idRaw, _ := args["observation_id"].(string)
	id, err := uuid.Parse(idRaw)
	if err != nil {
		return mcp.NewToolResultError("observation_id inválido"), nil
	}
	orgID, err := uuid.Parse(d.Principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	// Guard anti-enumeration: misma respuesta para no-existe y otra org.
	// Con RLS, la query Get de otra org devuelve ErrNotFound (0 rows), no leak.
	obs, err := d.Observations.Get(ctx, id)
	if err != nil || obs.OrganizationID != orgID {
		return mcp.NewToolResultError("observation not found"), nil
	}
	userID, _ := uuid.Parse(d.Principal.UserID)
	if err := d.Observations.SoftDelete(ctx, id, userID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("delete failed: %v", err)), nil
	}
	_ = commitOrRollback(ctx, tx, nil)
	return toolResultJSON(map[string]any{"deleted": true, "observation_id": id})
}

func toolMemSavePrompt() mcp.Tool {
	return mcp.NewTool("domain_mem_save_prompt",
		mcp.WithDescription("Guarda un prompt reutilizable versionado (por slug). Cada save del mismo slug crea una versión nueva."),
		mcp.WithString("slug",
			mcp.Description("Slug estable del prompt (kebab-case)"),
			mcp.Required(),
		),
		mcp.WithString("body",
			mcp.Description("Cuerpo del prompt (puede incluir {{variables}})"),
			mcp.Required(),
		),
		mcp.WithString("project_slug",
			mcp.Description("Project al que scoping el prompt (opcional: global de la org si se omite)"),
		),
		mcp.WithString("description",
			mcp.Description("Descripción corta"),
		),
		mcp.WithBoolean("set_active",
			mcp.Description("Marcar esta versión como activa (default true)"),
		),
	)
}

func (d *Deps) handleMemSavePrompt(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	args := req.GetArguments()
	slug, _ := args["slug"].(string)
	body, _ := args["body"].(string)
	if slug == "" || body == "" {
		return mcp.NewToolResultError("slug y body son requeridos"), nil
	}
	orgID, err := uuid.Parse(d.Principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	userID, _ := uuid.Parse(d.Principal.UserID)

	var projectID *uuid.UUID
	if ps, _ := args["project_slug"].(string); ps != "" {
		proj, err := d.Projects.GetBySlug(ctx, orgID, ps)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", ps)), nil
		}
		projectID = &proj.ID
	}
	desc, _ := args["description"].(string)
	setActive := true
	if v, ok := args["set_active"].(bool); ok {
		setActive = v
	}

	p, err := d.Prompts.Create(ctx, promptsvc.CreateInput{
		OrganizationID: orgID, ProjectID: projectID, CreatedBy: &userID,
		Slug: slug, Body: body, Description: desc, SetActive: setActive,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("save prompt failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"prompt_id": p.ID, "slug": p.Slug, "version": p.Version, "active": p.IsActive,
	})
}

func toolMemCapturePassive() mcp.Tool {
	return mcp.NewTool("domain_mem_capture_passive",
		mcp.WithDescription("Captura pasiva de contexto (baja prioridad): guarda una observación tipo 'passive' con dedup automático por hash de contenido."),
		mcp.WithString("project_slug",
			mcp.Description("Slug del project"),
			mcp.Required(),
		),
		mcp.WithString("content",
			mcp.Description("Contenido capturado"),
			mcp.Required(),
		),
		mcp.WithString("source",
			mcp.Description("Origen de la captura (ej: 'conversation', 'tool_output')"),
		),
	)
}

func (d *Deps) handleMemCapturePassive(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	// issue-25.14: wireup tx con SET LOCAL para RLS
	ctx, tx, release := withOrgCtx(ctx, d.Pool, d.Principal)
	defer release()
	args := req.GetArguments()
	projectSlug, _ := args["project_slug"].(string)
	content, _ := args["content"].(string)
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
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", projectSlug)), nil
	}
	source, _ := args["source"].(string)
	if source == "" {
		source = "passive"
	}

	obs, err := d.Observations.Save(ctx, obssvc.SaveInput{
		OrganizationID:  orgID,
		ProjectID:       proj.ID,
		CreatedBy:       &userID,
		Content:         content,
		ObservationType: "passive",
		Metadata:        map[string]any{"source": source, "passive": true},
	})
	if err != nil {
		// Dedup hash: contenido idéntico ya capturado no es error para el caller.
		if strings.Contains(err.Error(), "duplicate") {
			return toolResultJSON(map[string]any{"captured": false, "reason": "duplicate"})
		}
		return mcp.NewToolResultError(fmt.Sprintf("capture failed: %v", err)), nil
	}
	_ = commitOrRollback(ctx, tx, nil)
	return toolResultJSON(map[string]any{"captured": true, "observation_id": obs.ID})
}

func toolMemSuggestTopicKey() mcp.Tool {
	return mcp.NewTool("domain_mem_suggest_topic_key",
		mcp.WithDescription("Sugiere un topic_key kebab-case estable a partir de un contenido (heurística de keywords, sin LLM)."),
		mcp.WithString("content",
			mcp.Description("Contenido del cual derivar el topic key"),
			mcp.Required(),
		),
	)
}

var (
	reWord = regexp.MustCompile(`[a-záéíóúñü0-9]+`)
	// stopwords ES+EN mínimas para keywords.
	topicStopwords = map[string]bool{
		"el": true, "la": true, "los": true, "las": true, "de": true, "del": true,
		"en": true, "un": true, "una": true, "que": true, "con": true, "por": true,
		"para": true, "se": true, "su": true, "al": true, "es": true, "y": true,
		"o": true, "the": true, "a": true, "an": true, "of": true, "to": true,
		"in": true, "on": true, "for": true, "and": true, "or": true, "is": true,
		"this": true, "that": true, "it": true, "as": true, "be": true, "we": true,
	}
)

// SuggestTopicKey deriva un slug kebab-case con las keywords más frecuentes.
func SuggestTopicKey(content string) string {
	words := reWord.FindAllString(strings.ToLower(content), -1)
	freq := map[string]int{}
	order := []string{}
	for _, w := range words {
		if len(w) < 3 || topicStopwords[w] {
			continue
		}
		if freq[w] == 0 {
			order = append(order, w)
		}
		freq[w]++
	}
	if len(order) == 0 {
		return "general"
	}
	// Orden estable: frecuencia desc, luego orden de aparición.
	pos := map[string]int{}
	for i, w := range order {
		pos[w] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		if freq[order[i]] != freq[order[j]] {
			return freq[order[i]] > freq[order[j]]
		}
		return pos[order[i]] < pos[order[j]]
	})
	n := 3
	if len(order) < n {
		n = len(order)
	}
	return strings.Join(order[:n], "-")
}

func (d *Deps) handleMemSuggestTopicKey(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	content, _ := args["content"].(string)
	if content == "" {
		return mcp.NewToolResultError("content es requerido"), nil
	}
	return toolResultJSON(map[string]any{"topic_key": SuggestTopicKey(content)})
}

func toolMemStats() mcp.Tool {
	return mcp.NewTool("domain_mem_stats",
		mcp.WithDescription("Estadísticas de memoria de la org: observations totales, por tipo, sessions y prompts."),
		mcp.WithString("project_slug",
			mcp.Description("Limitar stats a un project (opcional)"),
		),
	)
}

func (d *Deps) handleMemStats(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	orgID, err := uuid.Parse(d.Principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	args := req.GetArguments()

	projFilter := ""
	qArgs := []any{orgID}
	if ps, _ := args["project_slug"].(string); ps != "" {
		proj, err := d.Projects.GetBySlug(ctx, orgID, ps)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", ps)), nil
		}
		projFilter = " AND project_id = $2"
		qArgs = append(qArgs, proj.ID)
	}

	byType := map[string]int64{}
	rows, err := d.Pool.Query(ctx, `
		SELECT observation_type, COUNT(*) FROM observations
		WHERE organization_id = $1 AND deleted_at IS NULL`+projFilter+`
		GROUP BY observation_type`, qArgs...)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("stats query failed: %v", err)), nil
	}
	var total int64
	for rows.Next() {
		var typ string
		var n int64
		if err := rows.Scan(&typ, &n); err != nil {
			rows.Close()
			return mcp.NewToolResultError(err.Error()), nil
		}
		byType[typ] = n
		total += n
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	var sessions, prompts int64
	_ = d.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM sessions WHERE organization_id = $1`, orgID).Scan(&sessions)
	_ = d.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM prompts WHERE organization_id = $1 AND deleted_at IS NULL`, orgID).Scan(&prompts)

	return toolResultJSON(map[string]any{
		"observations_total":   total,
		"observations_by_type": byType,
		"sessions_total":       sessions,
		"prompts_total":        prompts,
	})
}
