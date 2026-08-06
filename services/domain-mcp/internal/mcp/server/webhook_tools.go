// DOMAINSERV-240 — camino de alta de webhooks inbound.
//
// internal/service/webhook tenía Create, List, SetEnabled, SoftDelete, Deliveries y
// GetDelivery escritos, y su único llamador era webhook_integration_test.go: ni endpoint
// REST, ni tool MCP, ni pantalla en domain-admin. La consecuencia no era una feature a
// medias sino una feature INALCANZABLE: con DOMAIN_MASTER_KEY puesta,
// POST /api/v1/webhooks/{slug}/receive dejaba de dar 503 webhooks_disabled y pasaba a
// dar 404 para CUALQUIER slug, porque no existía forma de dar de alta un webhook.
//
// La superficie elegida es MCP y no REST ni UI: es la que usa el resto de la
// plataforma (crons, clients, agents, tickets se crean todos por tool), así que no
// introduce un segundo modelo de autenticación ni una pantalla que mantener.
package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/auth/apikey"
	webhooksvc "nunezlagos/domain/internal/service/webhook"
)

type inboundWebhookService interface {
	Create(ctx context.Context, in webhooksvc.CreateInput) (*webhooksvc.Webhook, error)
	List(ctx context.Context, orgID uuid.UUID) ([]webhooksvc.Webhook, error)
	SetEnabled(ctx context.Context, id uuid.UUID, enabled bool) error
	SoftDelete(ctx context.Context, id, actorID uuid.UUID) error
	Deliveries(ctx context.Context, webhookID uuid.UUID, limit int) ([]webhooksvc.Delivery, error)
}

type webhookHandlers struct {
	webhooks  inboundWebhookService
	projectID func(ctx context.Context, orgID uuid.UUID, slug string) (uuid.UUID, error)
	principal *apikey.Principal
}

func registerWebhookTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	h := &webhookHandlers{principal: deps.Principal, projectID: deps.projectIDConScope}
	// la asignación es condicional por el typed-nil de Go: un *Service nil guardado en un
	// campo de interfaz NO compara igual a nil, así que el guard de "service not
	// configured" no lo atraparía y el primer Create paniquearía al tocar s.Crypto
	if deps.InboundWebhooks != nil {
		h.webhooks = deps.InboundWebhooks
	}
	// TODAS pasan por rlsProyecto y no por el wrapper de org: webhooks está bajo RLS por
	// app.current_project_id desde la 000288, así que sin el GUC seteado el management
	// devuelve CERO filas sin error y el alta viola el WITH CHECK. El eje es el proyecto y
	// no la organización porque el org de esta instancia es decorativo: apikey/store.go fija
	// un canonicalOrgID para toda credencial, así que un RLS por org no aislaría nada.
	rlsProyecto := func(fn mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withProjectTxHandler(&deps, fn)
	}
	return []mcpgo.ServerTool{
		{Tool: toolWebhookCreate(), Handler: wrap.Wrap("domain_webhook_create", rlsProyecto(h.handleWebhookCreate))},
		{Tool: toolWebhookList(), Handler: wrap.Wrap("domain_webhook_list", rlsProyecto(h.handleWebhookList))},
		{Tool: toolWebhookSetEnabled(), Handler: wrap.Wrap("domain_webhook_set_enabled", rlsProyecto(h.handleWebhookSetEnabled))},
		{Tool: toolWebhookDelete(), Handler: wrap.Wrap("domain_webhook_delete", rlsProyecto(h.handleWebhookDelete))},
		{Tool: toolWebhookDeliveries(), Handler: wrap.Wrap("domain_webhook_deliveries", rlsProyecto(h.handleWebhookDeliveries))},
	}
}

func toolWebhookCreate() mcp.Tool {
	return mcp.NewTool("domain_webhook_create",
		mcp.WithDescription("Da de alta un webhook inbound: un POST externo a /api/v1/webhooks/{slug}/receive dispara un flow/agent/skill existente. El secret se cifra at-rest y NUNCA se devuelve — guardalo al crearlo porque no hay forma de recuperarlo. La firma se verifica segun source_type: github usa X-Hub-Signature-256, gitlab compara X-Gitlab-Token, generic usa X-Domain-Signature; bitbucket NO valida firma y por eso esta rechazado."),
		mcp.WithString("project_slug", mcp.Description("Proyecto dueño del webhook. Es el eje de scope: sin el, el alta falla"), mcp.Required()),
		mcp.WithString("slug", mcp.Description("Slug unico del webhook (kebab-case, empieza con letra). Es lo que va en la URL publica. Es unico en TODA la instancia, no por proyecto, porque el endpoint publico resuelve solo por slug"), mcp.Required()),
		mcp.WithString("name", mcp.Description("Nombre legible"), mcp.Required()),
		mcp.WithString("secret", mcp.Description("Secret compartido con quien emite el webhook. Se cifra at-rest y no se puede leer despues"), mcp.Required()),
		mcp.WithString("source_type", mcp.Description("Quien emite: generic | github | gitlab"), mcp.Required()),
		mcp.WithString("target_type", mcp.Description("Que disparar: flow | agent | skill"), mcp.Required()),
		mcp.WithString("target_id", mcp.Description("UUID del flow/agent/skill a ejecutar"), mcp.Required()),
		mcp.WithObject("inputs_mapping", mcp.Description("Mapeo de campos del payload a inputs del target, con paths tipo JSONPath: {\"rama\": \"$.ref\", \"autor\": \"$.pusher.name\"}")),
	)
}

func toolWebhookList() mcp.Tool {
	return mcp.NewTool("domain_webhook_list",
		mcp.WithDescription("Lista los webhooks inbound del proyecto, sin sus secrets. Devuelve slug, target y si esta habilitado."),
		mcp.WithString("project_slug", mcp.Description("Proyecto cuyos webhooks listar"), mcp.Required()),
	)
}

func toolWebhookSetEnabled() mcp.Tool {
	return mcp.NewTool("domain_webhook_set_enabled",
		mcp.WithDescription("Pausa o reanuda un webhook. Deshabilitado, /receive responde igual que para un slug inexistente: no revela que el webhook existe."),
		mcp.WithString("project_slug", mcp.Description("Proyecto dueño del webhook"), mcp.Required()),
		mcp.WithString("id", mcp.Description("UUID del webhook"), mcp.Required()),
		mcp.WithBoolean("enabled", mcp.Description("true para reanudar, false para pausar"), mcp.Required()),
	)
}

func toolWebhookDelete() mcp.Tool {
	return mcp.NewTool("domain_webhook_delete",
		mcp.WithDescription("Soft-delete de un webhook. El historial de entregas se preserva; el endpoint deja de aceptar. El slug queda libre para reusarse: delete + create es el camino de rotacion del secret."),
		mcp.WithString("project_slug", mcp.Description("Proyecto dueño del webhook"), mcp.Required()),
		mcp.WithString("id", mcp.Description("UUID del webhook"), mcp.Required()),
	)
}

func toolWebhookDeliveries() mcp.Tool {
	return mcp.NewTool("domain_webhook_deliveries",
		mcp.WithDescription("Historial de entregas de un webhook, mas reciente primero. Para diagnosticar firmas que no cuadran: el valor de los headers sensibles sale redactado, la clave se conserva. Una entrega con firma invalida se registra SIN payload."),
		mcp.WithString("project_slug", mcp.Description("Proyecto dueño del webhook"), mcp.Required()),
		mcp.WithString("id", mcp.Description("UUID del webhook"), mcp.Required()),
		mcp.WithNumber("limit", mcp.Description("Cuantas entregas devolver (default 50, max 200)")),
	)
}

// bitbucket se rechaza en el alta a propósito: verifyWebhookSignature devolvía true sin
// mirar nada para ese source_type, así que un webhook bitbucket aceptaría cualquier
// payload de cualquiera. Mientras no haya verificación de firma implementada, la
// prevención va en el camino de alta y no solo en el de recepción.
var sourcesConVerificacionDeFirma = map[string]bool{
	"generic": true,
	"github":  true,
	"gitlab":  true,
}

// argsDeAlta valida los argumentos del alta y devuelve el input sin el proyecto, que se
// resuelve aparte. Separado del handler para que ninguno de los dos pase de 50 líneas.
func argsDeAlta(req mcp.CallToolRequest) (webhooksvc.CreateInput, error) {
	args := req.GetArguments()
	slug, _ := args["slug"].(string)
	name, _ := args["name"].(string)
	secret, _ := args["secret"].(string)
	sourceType, _ := args["source_type"].(string)
	targetType, _ := args["target_type"].(string)
	targetIDStr, _ := args["target_id"].(string)
	if slug == "" || name == "" || secret == "" || sourceType == "" || targetType == "" || targetIDStr == "" {
		return webhooksvc.CreateInput{}, fmt.Errorf(
			"slug, name, secret, source_type, target_id y target_type son requeridos")
	}
	targetID, err := uuid.Parse(targetIDStr)
	if err != nil {
		return webhooksvc.CreateInput{}, fmt.Errorf("target_id invalido (UUID requerido)")
	}
	sourceType = strings.ToLower(strings.TrimSpace(sourceType))
	if !sourcesConVerificacionDeFirma[sourceType] {
		return webhooksvc.CreateInput{}, fmt.Errorf(
			"source_type '%s' no se puede dar de alta: no tiene verificacion de firma "+
				"implementada, asi que aceptaria cualquier payload. Usá generic, github o gitlab",
			sourceType)
	}
	in := webhooksvc.CreateInput{
		Slug:       slug,
		Name:       name,
		Secret:     secret,
		SourceType: sourceType,
		TargetType: strings.ToLower(strings.TrimSpace(targetType)),
		TargetID:   targetID,
	}
	if v, ok := args["inputs_mapping"].(map[string]any); ok {
		in.InputsMapping = v
	}
	return in, nil
}

func (h *webhookHandlers) handleWebhookCreate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.listo(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	in, err := argsDeAlta(req)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	userID, _ := uuid.Parse(h.principal.UserID)
	// el mismo project que withProjectTxHandler ya dejó en el GUC: resolverlo otra vez es lo
	// que hace que la fila cumpla WITH CHECK (project_id = current_project_id()). Si no
	// coincidieran, el insert fallaría con violación de la policy en vez de escribir mal
	projectID, err := h.projectID(ctx, orgID, argString(req, "project_slug"))
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project_slug: %v", err)), nil
	}
	in.OrganizationID, in.ProjectID, in.CreatedBy, in.ActorID = orgID, projectID, &userID, userID

	hook, err := h.webhooks.Create(ctx, in)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("create webhook failed: %v", err)), nil
	}
	// el secret no se devuelve ni siquiera acá: si se pierde, se rota con delete + create
	return toolResultJSON(map[string]any{
		"id":           hook.ID.String(),
		"slug":         hook.Slug,
		"receive_path": "/api/v1/webhooks/" + hook.Slug + "/receive",
		"source_type":  hook.SourceType,
		"target_type":  hook.TargetType,
		"enabled":      hook.Enabled,
	})
}

func (h *webhookHandlers) handleWebhookList(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.listo(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	orgID, _ := uuid.Parse(h.principal.OrganizationID)
	hooks, err := h.webhooks.List(ctx, orgID)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list webhooks failed: %v", err)), nil
	}
	out := make([]map[string]any, len(hooks))
	for i, hook := range hooks {
		out[i] = map[string]any{
			"id":           hook.ID.String(),
			"slug":         hook.Slug,
			"name":         hook.Name,
			"source_type":  hook.SourceType,
			"target_type":  hook.TargetType,
			"target_id":    hook.TargetID.String(),
			"enabled":      hook.Enabled,
			"receive_path": "/api/v1/webhooks/" + hook.Slug + "/receive",
		}
	}
	return toolResultJSON(map[string]any{"webhooks": out, "count": len(out)})
}

func (h *webhookHandlers) handleWebhookSetEnabled(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.listo(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	id, err := uuid.Parse(argString(req, "id"))
	if err != nil {
		return mcp.NewToolResultError("id invalido (UUID requerido)"), nil
	}
	enabled, _ := req.GetArguments()["enabled"].(bool)
	if err := h.webhooks.SetEnabled(ctx, id, enabled); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("set_enabled failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"id": id.String(), "enabled": enabled})
}

func (h *webhookHandlers) handleWebhookDelete(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.listo(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	id, err := uuid.Parse(argString(req, "id"))
	if err != nil {
		return mcp.NewToolResultError("id invalido (UUID requerido)"), nil
	}
	actorID, _ := uuid.Parse(h.principal.UserID)
	if err := h.webhooks.SoftDelete(ctx, id, actorID); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("delete failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"id": id.String(), "deleted": true})
}

func (h *webhookHandlers) handleWebhookDeliveries(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.listo(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	id, err := uuid.Parse(argString(req, "id"))
	if err != nil {
		return mcp.NewToolResultError("id invalido (UUID requerido)"), nil
	}
	limit := 50
	if v, ok := req.GetArguments()["limit"].(float64); ok {
		limit = int(v)
	}
	deliveries, err := h.webhooks.Deliveries(ctx, id, limit)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("deliveries failed: %v", err)), nil
	}
	return toolResultJSON(map[string]any{"deliveries": deliveries, "count": len(deliveries)})
}

func (h *webhookHandlers) listo() error {
	if h.principal == nil {
		return fmt.Errorf("no authenticated principal")
	}
	if h.webhooks == nil {
		return fmt.Errorf("webhooks service not configured")
	}
	return nil
}

func argString(req mcp.CallToolRequest, key string) string {
	s, _ := req.GetArguments()[key].(string)
	return s
}
