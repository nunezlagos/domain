// Tools MCP de clients (mandantes). Util para consultoras que gestionan
// proyectos por cliente. Tools registradas con prefijo `domain_client_*`.
//
// Tools:
//   - domain_client_create       crea un cliente en la org actual
//   - domain_client_list         lista clientes (filtros: status/limit/offset)
//   - domain_client_get          obtiene por id o slug
//   - domain_client_update       update parcial por id
//   - domain_client_delete       soft-delete por id
//   - domain_client_restore      revierte soft-delete por id
//   - domain_client_set_status   cambia el status del cliente
//
// Convenciones:
//   - Reutiliza ClientService (mismo path que los handlers REST).
//   - Principal del MCP server resuelto al boot (DOMAIN_API_KEY).
//   - Cross-org guard: GetByID compara OrganizationID antes de exponer.
package mcpserver

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	clientsvc "nunezlagos/domain/internal/service/client"
)

func registerClientTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {



	rls := func(h mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withOrgTxHandler(&deps, h)
	}
	return []mcpgo.ServerTool{
		{Tool: toolClientCreate(), Handler: wrap.Wrap("domain_client_create", rls(deps.handleClientCreate))},
		{Tool: toolClientList(), Handler: wrap.Wrap("domain_client_list", rls(deps.handleClientList))},
		{Tool: toolClientGet(), Handler: wrap.Wrap("domain_client_get", rls(deps.handleClientGet))},
		{Tool: toolClientUpdate(), Handler: wrap.Wrap("domain_client_update", rls(deps.handleClientUpdate))},
		{Tool: toolClientDelete(), Handler: wrap.Wrap("domain_client_delete", rls(deps.handleClientDelete))},
		{Tool: toolClientRestore(), Handler: wrap.Wrap("domain_client_restore", rls(deps.handleClientRestore))},
		{Tool: toolClientSetStatus(), Handler: wrap.Wrap("domain_client_set_status", rls(deps.handleClientSetStatus))},
	}
}



func toolClientCreate() mcp.Tool {
	return mcp.NewTool("domain_client_create",
		mcp.WithDescription("Crea un cliente/mandante en la org actual del principal. Util para consultoras que gestionan proyectos por cliente."),
		mcp.WithString("name",
			mcp.Description("Nombre del cliente (razon social o nombre comercial)"),
			mcp.Required(),
		),
		mcp.WithString("slug",
			mcp.Description("Slug unico del cliente dentro de la org (kebab-case)"),
			mcp.Required(),
		),
		mcp.WithString("tax_id",
			mcp.Description("Tax ID / RUT / CUIT (opcional)"),
		),
		mcp.WithString("contact_email",
			mcp.Description("Email de contacto principal (opcional)"),
		),
		mcp.WithString("contact_phone",
			mcp.Description("Telefono de contacto principal (opcional)"),
		),
		mcp.WithString("address",
			mcp.Description("Direccion postal (opcional)"),
		),
		mcp.WithObject("metadata",
			mcp.Description("Metadata estructurada arbitraria (JSONB)"),
		),
	)
}

func toolClientList() mcp.Tool {
	return mcp.NewTool("domain_client_list",
		mcp.WithDescription("Lista los clientes/mandantes de la org actual. Filtros opcionales por status y paginacion con limit/offset."),
		mcp.WithString("status",
			mcp.Description("Filtrar por status (ej: active, archived). Vacio = todos."),
		),
		mcp.WithNumber("limit",
			mcp.Description("Maximo resultados (default 50, max 200)"),
		),
		mcp.WithNumber("offset",
			mcp.Description("Offset para paginacion (default 0)"),
		),
	)
}

func toolClientGet() mcp.Tool {
	return mcp.NewTool("domain_client_get",
		mcp.WithDescription("Obtiene un cliente por id (UUID) o slug. Scope: org actual del principal."),
		mcp.WithString("id_or_slug",
			mcp.Description("UUID del cliente o su slug dentro de la org"),
			mcp.Required(),
		),
	)
}

func toolClientUpdate() mcp.Tool {
	return mcp.NewTool("domain_client_update",
		mcp.WithDescription("Update parcial de un cliente. Solo los campos provistos en `fields` se actualizan."),
		mcp.WithString("id",
			mcp.Description("UUID del cliente a actualizar"),
			mcp.Required(),
		),
		mcp.WithObject("fields",
			mcp.Description(`Campos a actualizar: {"name":"...","tax_id":"...","contact_email":"...","contact_phone":"...","address":"...","metadata":{...}}`),
			mcp.Required(),
		),
	)
}

func toolClientDelete() mcp.Tool {
	return mcp.NewTool("domain_client_delete",
		mcp.WithDescription("Soft-delete (deleted_at=now) de un cliente. Se puede revertir con domain_client_restore."),
		mcp.WithString("id",
			mcp.Description("UUID del cliente"),
			mcp.Required(),
		),
	)
}

func toolClientRestore() mcp.Tool {
	return mcp.NewTool("domain_client_restore",
		mcp.WithDescription("Restaura un cliente previamente soft-deleted (deleted_at=NULL)."),
		mcp.WithString("id",
			mcp.Description("UUID del cliente"),
			mcp.Required(),
		),
	)
}

func toolClientSetStatus() mcp.Tool {
	return mcp.NewTool("domain_client_set_status",
		mcp.WithDescription("Cambia el status del cliente (ej: active, archived). Valida contra los estados permitidos del service."),
		mcp.WithString("id",
			mcp.Description("UUID del cliente"),
			mcp.Required(),
		),
		mcp.WithString("status",
			mcp.Description("Nuevo status (active | archived | ...)"),
			mcp.Required(),
		),
	)
}



func (d *Deps) handleClientCreate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal (set DOMAIN_API_KEY)"), nil
	}
	if d.Clients == nil {
		return mcp.NewToolResultError("client service no configurado"), nil
	}
	args := req.GetArguments()
	name, _ := args["name"].(string)
	slug, _ := args["slug"].(string)
	if name == "" || slug == "" {
		return mcp.NewToolResultError("name y slug son requeridos"), nil
	}
	orgID, err := uuid.Parse(d.Principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	userID, _ := uuid.Parse(d.Principal.UserID)
	taxID, _ := args["tax_id"].(string)
	email, _ := args["contact_email"].(string)
	phone, _ := args["contact_phone"].(string)
	address, _ := args["address"].(string)
	metadata, _ := args["metadata"].(map[string]any)

	cl, err := d.Clients.Create(ctx, orgID, clientsvc.CreateInput{
		Name:         name,
		Slug:         slug,
		TaxID:        taxID,
		ContactEmail: email,
		ContactPhone: phone,
		Address:      address,
		Metadata:     metadata,
		ActorID:      &userID,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create client failed: %v", err)), nil
	}
	return toolResultJSON(cl)
}

func (d *Deps) handleClientList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if d.Clients == nil {
		return mcp.NewToolResultError("client service no configurado"), nil
	}
	orgID, err := uuid.Parse(d.Principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	args := req.GetArguments()
	status, _ := args["status"].(string)
	limit := 0
	if v, ok := args["limit"].(float64); ok {
		limit = int(v)
	}
	offset := 0
	if v, ok := args["offset"].(float64); ok {
		offset = int(v)
	}
	list, total, err := d.Clients.List(ctx, orgID, clientsvc.ListFilter{
		Status: status, Limit: limit, Offset: offset,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list clients failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"clients": list, "total": total})
}

func (d *Deps) handleClientGet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if d.Clients == nil {
		return mcp.NewToolResultError("client service no configurado"), nil
	}
	args := req.GetArguments()
	idOrSlug, _ := args["id_or_slug"].(string)
	if idOrSlug == "" {
		return mcp.NewToolResultError("id_or_slug requerido"), nil
	}
	orgID, err := uuid.Parse(d.Principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	cl, err := lookupClientByIDOrSlug(ctx, d.Clients, orgID, idOrSlug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("not found: %v", err)), nil
	}
	return toolResultJSON(cl)
}

func (d *Deps) handleClientUpdate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if d.Clients == nil {
		return mcp.NewToolResultError("client service no configurado"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id invalido (UUID requerido)"), nil
	}
	fields, _ := args["fields"].(map[string]any)
	if len(fields) == 0 {
		return mcp.NewToolResultError("fields es requerido y no puede estar vacio"), nil
	}
	orgID, err := uuid.Parse(d.Principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	userID, _ := uuid.Parse(d.Principal.UserID)
	if _, err := d.Clients.Get(ctx, orgID, id.String()); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("not found: %v", err)), nil
	}
	in := clientsvc.UpdateInput{ActorID: &userID}
	if v, ok := fields["name"].(string); ok {
		in.Name = &v
	}
	if v, ok := fields["tax_id"].(string); ok {
		in.TaxID = &v
	}
	if v, ok := fields["contact_email"].(string); ok {
		in.ContactEmail = &v
	}
	if v, ok := fields["contact_phone"].(string); ok {
		in.ContactPhone = &v
	}
	if v, ok := fields["address"].(string); ok {
		in.Address = &v
	}
	if v, ok := fields["metadata"].(map[string]any); ok {
		in.Metadata = v
	}
	cl, err := d.Clients.Update(ctx, orgID, id, in)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("update client failed: %v", err)), nil
	}
	return toolResultJSON(cl)
}

func (d *Deps) handleClientDelete(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if d.Clients == nil {
		return mcp.NewToolResultError("client service no configurado"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id invalido (UUID requerido)"), nil
	}
	orgID, err := uuid.Parse(d.Principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	_ = uuid.Nil // placeholder for backward compat — userID no usado directo en SoftDelete
	if _, err := d.Clients.Get(ctx, orgID, id.String()); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("not found: %v", err)), nil
	}
	if err := d.Clients.Delete(ctx, orgID, id); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("delete client failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"id": id.String(), "deleted": true})
}

func (d *Deps) handleClientRestore(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if d.Clients == nil {
		return mcp.NewToolResultError("client service no configurado"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id invalido (UUID requerido)"), nil
	}
	orgID, err := uuid.Parse(d.Principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}




	if err := d.Clients.Restore(ctx, orgID, id); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("restore client failed: %v", err)), nil
	}
	cl, err := d.Clients.Get(ctx, orgID, id.String())
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("reload after restore failed: %v", err)), nil
	}
	return toolResultJSON(cl)
}

func (d *Deps) handleClientSetStatus(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil {
		return mcp.NewToolResultError("no authenticated principal"), nil
	}
	if d.Clients == nil {
		return mcp.NewToolResultError("client service no configurado"), nil
	}
	args := req.GetArguments()
	idStr, _ := args["id"].(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return mcp.NewToolResultError("id invalido (UUID requerido)"), nil
	}
	status, _ := args["status"].(string)
	if status == "" {
		return mcp.NewToolResultError("status requerido"), nil
	}
	orgID, err := uuid.Parse(d.Principal.OrganizationID)
	if err != nil {
		return mcp.NewToolResultError("invalid principal org_id"), nil
	}
	if _, err := d.Clients.Get(ctx, orgID, id.String()); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("not found: %v", err)), nil
	}
	cl, err := d.Clients.SetStatus(ctx, orgID, id, status)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("set_status failed: %v", err)), nil
	}
	return toolResultJSON(cl)
}

// lookupClientByIDOrSlug delega en Service.Get (que ya parsea UUID vs slug
// internamente y aplica el filtro por orgID).
func lookupClientByIDOrSlug(ctx context.Context, svc *clientsvc.Service, orgID uuid.UUID, idOrSlug string) (*clientsvc.Client, error) {
	return svc.Get(ctx, orgID, idOrSlug)
}
