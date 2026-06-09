// MCP tools para external provider sync — HU-04.9

package mcpserver

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"
)

// toolSyncRegisterProvider — domain_sync_register_provider
func toolSyncRegisterProvider() mcp.Tool {
	return mcp.NewTool("domain_sync_register_provider",
		mcp.WithDescription("Registra o actualiza un proveedor externo (Jira, GitHub, Linear, Asana)."),
		mcp.WithString("provider",
			mcp.Description("Nombre del proveedor: jira | github | linear | asana"),
			mcp.Required(),
		),
		mcp.WithString("display_name",
			mcp.Description("Nombre visible del proveedor"),
			mcp.Required(),
		),
		mcp.WithString("base_url",
			mcp.Description("URL base de la API del proveedor"),
		),
		mcp.WithString("project_key",
			mcp.Description("Key del proyecto en el proveedor (ej. PROJ)"),
		),
		mcp.WithObject("config",
			mcp.Description("Config adicional del proveedor (JSONB)"),
		),
	)
}

func toolSyncRegisterPush() mcp.Tool {
	return mcp.NewTool("domain_sync_register_push",
		mcp.WithDescription("Registra un sync state tras un push exitoso a un proveedor externo."),
		mcp.WithString("provider_id",
			mcp.Description("UUID del proveedor"),
			mcp.Required(),
		),
		mcp.WithString("entity_kind",
			mcp.Description("Tipo de entidad: req | hu"),
			mcp.Required(),
		),
		mcp.WithString("entity_id",
			mcp.Description("UUID de la entidad en domain"),
			mcp.Required(),
		),
		mcp.WithString("external_key",
			mcp.Description("ID del recurso en el sistema externo"),
			mcp.Required(),
		),
		mcp.WithString("external_url",
			mcp.Description("URL del recurso en el sistema externo"),
		),
		mcp.WithString("external_type",
			mcp.Description("Tipo de recurso externo (ej. Epic, Story, Issue)"),
		),
		mcp.WithObject("field_mapping",
			mcp.Description("Mapeo de campos domain → externo"),
		),
	)
}

func toolSyncMarkDrift() mcp.Tool {
	return mcp.NewTool("domain_sync_mark_drift",
		mcp.WithDescription("Marca un sync state como conflicto por edición externa detectada."),
		mcp.WithString("state_id",
			mcp.Description("UUID del sync state"),
			mcp.Required(),
		),
		mcp.WithObject("drift_fields",
			mcp.Description("Campos con diferencias detectadas"),
		),
	)
}

func toolSyncMarkResolved() mcp.Tool {
	return mcp.NewTool("domain_sync_mark_resolved",
		mcp.WithDescription("Resuelve un conflicto marcado como drift."),
		mcp.WithString("state_id",
			mcp.Description("UUID del sync state"),
			mcp.Required(),
		),
	)
}

func toolSyncListConflicts() mcp.Tool {
	return mcp.NewTool("domain_sync_list_conflicts",
		mcp.WithDescription("Lista sync states con drift sin resolver."),
		mcp.WithNumber("limit",
			mcp.Description("Máximo resultados (default 50, max 200)"),
		),
	)
}

func toolSyncGetState() mcp.Tool {
	return mcp.NewTool("domain_sync_get_state",
		mcp.WithDescription("Recupera un sync state por ID."),
		mcp.WithString("id",
			mcp.Description("UUID del sync state"),
			mcp.Required(),
		),
	)
}

// --- handlers ---

func (d *Deps) handleSyncRegisterProvider(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.ExtSync == nil {
		return mcp.NewToolResultError("extsync service no configurado"), nil
	}
	args := req.GetArguments()
	provider, _ := args["provider"].(string)
	displayName, _ := args["display_name"].(string)
	if provider == "" || displayName == "" {
		return mcp.NewToolResultError("provider y display_name son requeridos"), nil
	}
	baseURL, _ := args["base_url"].(string)
	projectKey, _ := args["project_key"].(string)
	var config map[string]any
	if v, ok := args["config"].(map[string]any); ok {
		config = v
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	p, err := d.ExtSync.RegisterProvider(ctx, orgID, provider, displayName, baseURL, projectKey, config)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("register provider: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"id":           p.ID.String(),
		"provider":     p.Provider,
		"display_name": p.DisplayName,
		"enabled":      p.Enabled,
	})
}

func (d *Deps) handleSyncRegisterPush(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.ExtSync == nil {
		return mcp.NewToolResultError("extsync service no configurado"), nil
	}
	args := req.GetArguments()
	provIDStr, _ := args["provider_id"].(string)
	entityKind, _ := args["entity_kind"].(string)
	entityIDStr, _ := args["entity_id"].(string)
	extKey, _ := args["external_key"].(string)
	if provIDStr == "" || entityKind == "" || entityIDStr == "" || extKey == "" {
		return mcp.NewToolResultError("provider_id, entity_kind, entity_id y external_key son requeridos"), nil
	}
	provID, err := uuid.Parse(provIDStr)
	if err != nil {
		return mcp.NewToolResultError("provider_id inválido"), nil
	}
	entityID, err := uuid.Parse(entityIDStr)
	if err != nil {
		return mcp.NewToolResultError("entity_id inválido"), nil
	}
	extURL, _ := args["external_url"].(string)
	extType, _ := args["external_type"].(string)
	var fieldMapping map[string]any
	if v, ok := args["field_mapping"].(map[string]any); ok {
		fieldMapping = v
	}
	st, err := d.ExtSync.RegisterPush(ctx, provID, entityKind, entityID, extKey, extURL, extType, fieldMapping)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("register push: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"id":          st.ID.String(),
		"status":      st.SyncStatus,
		"external_key": st.ExternalKey,
		"direction":   st.SyncDirection,
	})
}

func (d *Deps) handleSyncMarkDrift(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.ExtSync == nil {
		return mcp.NewToolResultError("extsync service no configurado"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["state_id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("state_id inválido"), nil
	}
	var driftFields map[string]any
	if v, ok := args["drift_fields"].(map[string]any); ok {
		driftFields = v
	}
	st, err := d.ExtSync.MarkDrift(ctx, id, driftFields)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("mark drift: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"id":              st.ID.String(),
		"status":          st.SyncStatus,
		"drift_detected_at": st.DriftDetectedAt,
	})
}

func (d *Deps) handleSyncMarkResolved(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.ExtSync == nil {
		return mcp.NewToolResultError("extsync service no configurado"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["state_id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("state_id inválido"), nil
	}
	st, err := d.ExtSync.MarkResolved(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("mark resolved: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"id":     st.ID.String(),
		"status": st.SyncStatus,
	})
}

func (d *Deps) handleSyncListConflicts(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.ExtSync == nil {
		return mcp.NewToolResultError("extsync service no configurado"), nil
	}
	args := req.GetArguments()
	limit := 50
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}
	results, err := d.ExtSync.ListConflicts(ctx, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list conflicts: %v", err)), nil
	}
	out := make([]map[string]any, 0, len(results))
	for _, st := range results {
		out = append(out, map[string]any{
			"id":               st.ID.String(),
			"provider_id":      st.ProviderID.String(),
			"entity_kind":      st.EntityKind,
			"entity_id":        st.EntityID.String(),
			"external_key":     st.ExternalKey,
			"drift_detected_at": st.DriftDetectedAt,
		})
	}
	return toolResultJSON(map[string]any{"results": out, "count": len(out)})
}

func (d *Deps) handleSyncGetState(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.ExtSync == nil {
		return mcp.NewToolResultError("extsync service no configurado"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id inválido"), nil
	}
	st, err := d.ExtSync.Get(ctx, id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get: %v", err)), nil
	}
	return toolResultJSON(st)
}

// registerSyncTools agrega tools de external sync al listado.
func registerSyncTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	return []mcpgo.ServerTool{
		{Tool: toolSyncRegisterProvider(), Handler: wrap.Wrap("domain_sync_register_provider", deps.handleSyncRegisterProvider)},
		{Tool: toolSyncRegisterPush(), Handler: wrap.Wrap("domain_sync_register_push", deps.handleSyncRegisterPush)},
		{Tool: toolSyncMarkDrift(), Handler: wrap.Wrap("domain_sync_mark_drift", deps.handleSyncMarkDrift)},
		{Tool: toolSyncMarkResolved(), Handler: wrap.Wrap("domain_sync_mark_resolved", deps.handleSyncMarkResolved)},
		{Tool: toolSyncListConflicts(), Handler: wrap.Wrap("domain_sync_list_conflicts", deps.handleSyncListConflicts)},
		{Tool: toolSyncGetState(), Handler: wrap.Wrap("domain_sync_get_state", deps.handleSyncGetState)},
	}
}
