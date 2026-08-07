// issue-56.4 — tools de marcos normativos por proyecto.
//
// Existen desde el mismo commit que el service a propósito. La lección de DOMAINSERV-240 es que un
// service con métodos escritos y sin superficie de alta no es una feature a medias: es una feature
// INALCANZABLE, y el síntoma no se distingue de un bug.
//
// Scoping: el catálogo es global y se lista sin proyecto; todo lo demás pasa por rlsProyecto,
// porque las tablas por proyecto están bajo RLS por app.current_project_id y sin el GUC devuelven
// cero filas sin error.
package mcpserver

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/auth/apikey"
	compliancesvc "nunezlagos/domain/internal/service/compliance"
)

type complianceService interface {
	ListarCatalogo(ctx context.Context) ([]compliancesvc.Framework, error)
	MarcosDelProyecto(ctx context.Context, projectID uuid.UUID) ([]compliancesvc.Framework, error)
	DeclararMarco(ctx context.Context, projectID, frameworkID uuid.UUID, actorID *uuid.UUID, activo bool) error
	ControlesExigidos(ctx context.Context, projectID uuid.UUID, ahora time.Time) ([]compliancesvc.ControlExigido, error)
	RegistrarEstado(ctx context.Context, projectID, controlID uuid.UUID, estado, evidencia string, actorID *uuid.UUID) error
	BuscarPorSlug(ctx context.Context, slug, edicion string) (*compliancesvc.Framework, error)
	BuscarControlPorSlug(ctx context.Context, slug string) (uuid.UUID, error)
}

type complianceHandlers struct {
	compliance complianceService
	projectID  func(ctx context.Context, orgID uuid.UUID, slug string) (uuid.UUID, error)
	principal  *apikey.Principal
}

func registerComplianceTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	h := &complianceHandlers{principal: deps.Principal, projectID: deps.projectIDConScope}
	// condicional por el typed-nil de Go: un *Service nil en un campo de interfaz NO compara igual
	// a nil, así que el guard de "service no configurado" no lo atraparía
	if deps.Compliance != nil {
		h.compliance = deps.Compliance
	}
	rlsProyecto := func(fn mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withProjectTxHandler(&deps, fn)
	}
	return []mcpgo.ServerTool{
		// el catálogo es global: no lleva project_slug ni wrapper de proyecto
		{Tool: toolComplianceCatalog(), Handler: wrap.Wrap("domain_compliance_catalog", h.handleCatalog)},
		{Tool: toolComplianceProjectSet(), Handler: wrap.Wrap("domain_compliance_project_set", rlsProyecto(h.handleProjectSet))},
		{Tool: toolComplianceReport(), Handler: wrap.Wrap("domain_compliance_report", rlsProyecto(h.handleReport))},
		{Tool: toolComplianceControlSet(), Handler: wrap.Wrap("domain_compliance_control_set", rlsProyecto(h.handleControlSet))},
	}
}

func toolComplianceCatalog() mcp.Tool {
	return mcp.NewTool("domain_compliance_catalog",
		mcp.WithDescription("Lista el catalogo de marcos normativos que conoce la instancia: leyes (21.719, 21.595), reglamentos (GDPR) y normas tecnicas (ISO 27001). Es GLOBAL, no depende del proyecto. Cada marco dice si es obligatorio, si es certificable, desde cuando rige y si su texto se puede ingestar o solo referenciar por clausula."),
	)
}

func toolComplianceProjectSet() mcp.Tool {
	return mcp.NewTool("domain_compliance_project_set",
		mcp.WithDescription("Declara si un proyecto esta afecto a un marco normativo. SIN declaracion, el marco NO aplica: el default es que un proyecto no esta afecto a nada. Usar activo=false para dejar de aplicarlo sin perder el registro."),
		mcp.WithString("project_slug", mcp.Description("Proyecto que declara el marco"), mcp.Required()),
		mcp.WithString("framework_slug", mcp.Description("Slug del marco del catalogo (ej: ley-21719, gdpr, iso-27001)"), mcp.Required()),
		mcp.WithString("edicion", mcp.Description("Edicion del marco cuando hay mas de una (ej: '2022' para ISO 27001). Vacio si solo hay una")),
		mcp.WithBoolean("activo", mcp.Description("true para declararlo afecto (default), false para dejar de aplicarlo")),
	)
}

func toolComplianceReport() mcp.Tool {
	return mcp.NewTool("domain_compliance_report",
		mcp.WithDescription("Postura de cumplimiento del proyecto: que marcos declaro y que controles le exigen, cada uno con la referencia de articulo o clausula del marco que lo pide. Un control exigido por varios marcos aparece una vez por marco: se implementa UNA vez y se reporta en todos. Los marcos que todavia no rigen salen marcados como no vigentes."),
		mcp.WithString("project_slug", mcp.Description("Proyecto a reportar"), mcp.Required()),
	)
}

func toolComplianceControlSet() mcp.Tool {
	return mcp.NewTool("domain_compliance_control_set",
		mcp.WithDescription("Registra el estado de un control para el proyecto: ok | parcial | falta | no_verificable, con su evidencia. Es idempotente: re-evaluar el mismo control actualiza la fila en vez de acumular. no_verificable es un estado propio y no un incumplimiento: hay controles de gobernanza que el codigo no puede demostrar."),
		mcp.WithString("project_slug", mcp.Description("Proyecto dueño del estado"), mcp.Required()),
		mcp.WithString("control_slug", mcp.Description("Slug del control (ej: cifrado-en-reposo)"), mcp.Required()),
		mcp.WithString("estado", mcp.Description("ok | parcial | falta | no_verificable"), mcp.Required()),
		mcp.WithString("evidencia", mcp.Description("Con que se sostiene ese estado (path, commit, config)")),
	)
}

func (h *complianceHandlers) handleCatalog(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.compliance == nil {
		return mcp.NewToolResultError("compliance service no configurado"), nil
	}
	marcos, err := h.compliance.ListarCatalogo(ctx)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("catalogo: %v", err)), nil
	}
	ahora := time.Now()
	out := make([]map[string]any, len(marcos))
	for i, m := range marcos {
		out[i] = map[string]any{
			"slug": m.Slug, "nombre": m.Nombre, "tipo": m.Tipo,
			"jurisdiccion": m.Jurisdiccion, "obligatorio": m.Obligatorio,
			"certificable": m.Certificable, "edicion": m.Edicion,
			"vigente":               m.Vigente(ahora),
			"admite_texto_completo": m.AdmiteTextoCompleto(),
		}
	}
	return toolResultJSON(map[string]any{"frameworks": out, "count": len(out)})
}

func (h *complianceHandlers) handleProjectSet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	projectID, marco, errRes := h.resolverProyectoYMarco(ctx, req)
	if errRes != nil {
		return errRes, nil
	}
	activo := true
	if v, ok := req.GetArguments()["activo"].(bool); ok {
		activo = v
	}
	userID, _ := uuid.Parse(h.principal.UserID)
	if err := h.compliance.DeclararMarco(ctx, projectID, marco.ID, &userID, activo); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("declarar marco: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"framework_slug": marco.Slug, "activo": activo, "obligatorio": marco.Obligatorio,
		"vigente": marco.Vigente(time.Now()),
	})
}

func (h *complianceHandlers) handleReport(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.compliance == nil || h.principal == nil {
		return mcp.NewToolResultError("compliance service no configurado"), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	projectID, err := h.projectID(ctx, orgID, argString(req, "project_slug"))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project_slug: %v", err)), nil
	}
	ahora := time.Now()
	marcos, err := h.compliance.MarcosDelProyecto(ctx, projectID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marcos: %v", err)), nil
	}
	controles, err := h.compliance.ControlesExigidos(ctx, projectID, ahora)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("controles: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"marcos_declarados":  proyectarMarcos(marcos, ahora),
		"controles_exigidos": proyectarControles(controles),
		// sin marcos declarados no hay obligaciones: es el estado normal, no una falta de datos
		"aplica": len(marcos) > 0,
	})
}

func (h *complianceHandlers) handleControlSet(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if h.compliance == nil || h.principal == nil {
		return mcp.NewToolResultError("compliance service no configurado"), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	projectID, err := h.projectID(ctx, orgID, argString(req, "project_slug"))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project_slug: %v", err)), nil
	}
	controlID, errRes := h.resolverControl(ctx, argString(req, "control_slug"))
	if errRes != nil {
		return errRes, nil
	}
	userID, _ := uuid.Parse(h.principal.UserID)
	estado := argString(req, "estado")
	if err := h.compliance.RegistrarEstado(ctx, projectID, controlID, estado,
		argString(req, "evidencia"), &userID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("registrar estado: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"control_slug": argString(req, "control_slug"), "estado": estado,
	})
}

// resolverProyectoYMarco hace las dos resoluciones que comparten project_set: el proyecto contra
// la org y el marco contra el catálogo.
func (h *complianceHandlers) resolverProyectoYMarco(ctx context.Context, req mcp.CallToolRequest,
) (uuid.UUID, *compliancesvc.Framework, *mcp.CallToolResult) {
	if h.compliance == nil || h.principal == nil {
		return uuid.Nil, nil, mcp.NewToolResultError("compliance service no configurado")
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	projectID, err := h.projectID(ctx, orgID, argString(req, "project_slug"))
	if err != nil {
		return uuid.Nil, nil, mcp.NewToolResultError(fmt.Sprintf("project_slug: %v", err))
	}
	marco, err := h.compliance.BuscarPorSlug(ctx, argString(req, "framework_slug"), argString(req, "edicion"))
	if err != nil {
		return uuid.Nil, nil, mcp.NewToolResultError(fmt.Sprintf("framework_slug: %v", err))
	}
	return projectID, marco, nil
}

func (h *complianceHandlers) resolverControl(ctx context.Context, slug string) (uuid.UUID, *mcp.CallToolResult) {
	id, err := h.compliance.BuscarControlPorSlug(ctx, slug)
	if err != nil {
		return uuid.Nil, mcp.NewToolResultError(fmt.Sprintf("control_slug: %v", err))
	}
	return id, nil
}

func proyectarMarcos(marcos []compliancesvc.Framework, ahora time.Time) []map[string]any {
	out := make([]map[string]any, len(marcos))
	for i, m := range marcos {
		out[i] = map[string]any{
			"slug": m.Slug, "nombre": m.Nombre, "tipo": m.Tipo,
			"obligatorio": m.Obligatorio, "vigente": m.Vigente(ahora),
		}
	}
	return out
}

func proyectarControles(controles []compliancesvc.ControlExigido) []map[string]any {
	out := make([]map[string]any, len(controles))
	for i, c := range controles {
		out[i] = map[string]any{
			"control_slug": c.ControlSlug, "nombre": c.Nombre,
			"exigido_por": c.FrameworkSlug, "referencia": c.Referencia,
			"obligatorio": c.Obligatorio, "vigente": c.Vigente,
		}
	}
	return out
}
