// DOMAINSERV-79 H1 — Tools MCP para attachments (MinIO/S3). Envuelven el
// servicio existente internal/service/attachment (hasta ahora código muerto en
// prod: solo lo usaba el issuebuilder desde tests). El server NUNCA proxya
// bytes: init/get devuelven presigned URLs y el cliente sube/baja directo a S3.
// Corren bajo withOrgTxHandler (RLS por org) igual que los ticket tools.
package mcpserver

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"
	mcpgo "github.com/mark3labs/mcp-go/server"

	"nunezlagos/domain/internal/auth/apikey"
	attachmentsvc "nunezlagos/domain/internal/service/attachment"
)

// attachmentService: interface definida en el CONSUMIDOR (policy
// coupling-consumer-defined-interfaces). El *attachment.Service concreto la
// satisface; los tests inyectan un fake.
type attachmentService interface {
	InitUpload(ctx context.Context, entityType, entityID, filename, mimeType, createdBy string, size int64) (*attachmentsvc.InitUploadResult, error)
	ConfirmUpload(ctx context.Context, attachmentID uuid.UUID) (*attachmentsvc.ConfirmResult, error)
	GetDownloadURL(ctx context.Context, attachmentID uuid.UUID) (*attachmentsvc.ConfirmResult, error)
	ListByEntity(ctx context.Context, entityType, entityID string) ([]attachmentsvc.Attachment, error)
	Delete(ctx context.Context, attachmentID uuid.UUID) error
}

type attachmentHandlers struct {
	attachments attachmentService
	principal   *apikey.Principal
}

func registerAttachmentTools(wrap *ResilientWrapper, deps Deps) []mcpgo.ServerTool {
	h := &attachmentHandlers{
		attachments: deps.Attachments,
		principal:   deps.Principal,
	}
	rls := func(fn mcpgo.ToolHandlerFunc) mcpgo.ToolHandlerFunc {
		return withOrgTxHandler(&deps, fn)
	}
	return []mcpgo.ServerTool{
		{Tool: toolAttachmentInitUpload(), Handler: wrap.Wrap("domain_attachment_init_upload", rls(h.handleAttachmentInitUpload))},
		{Tool: toolAttachmentConfirm(), Handler: wrap.Wrap("domain_attachment_confirm", rls(h.handleAttachmentConfirm))},
		{Tool: toolAttachmentGetURL(), Handler: wrap.Wrap("domain_attachment_get_url", rls(h.handleAttachmentGetURL))},
		{Tool: toolAttachmentList(), Handler: wrap.Wrap("domain_attachment_list", rls(h.handleAttachmentList))},
		{Tool: toolAttachmentDelete(), Handler: wrap.Wrap("domain_attachment_delete", rls(h.handleAttachmentDelete))},
	}
}

const attachmentEntityDesc = "Tipo de entidad dueña del adjunto. Aceptados: user_story, requirement, hu_draft, intake_payload, ticket."

func toolAttachmentInitUpload() mcp.Tool {
	return mcp.NewTool("domain_attachment_init_upload",
		mcp.WithDescription("Inicia la subida de un adjunto a una entidad. Valida MIME (image/*, application/pdf, text/markdown, text/plain) y tamaño (máx 10MB), crea la fila en file_attachments y devuelve una presigned PUT URL (15min). El cliente sube los bytes DIRECTO a esa URL — el server no proxya. Tras subir, llamar domain_attachment_confirm."),
		mcp.WithString("entity_type", mcp.Description(attachmentEntityDesc), mcp.Required()),
		mcp.WithString("entity_id", mcp.Description("UUID de la entidad dueña"), mcp.Required()),
		mcp.WithString("filename", mcp.Description("Nombre del archivo (con extensión)"), mcp.Required()),
		mcp.WithString("mime_type", mcp.Description("MIME type del archivo (ej. application/pdf)"), mcp.Required()),
		mcp.WithNumber("size_bytes", mcp.Description("Tamaño del archivo en bytes (máx 10485760)"), mcp.Required()),
	)
}

func toolAttachmentConfirm() mcp.Tool {
	return mcp.NewTool("domain_attachment_confirm",
		mcp.WithDescription("Confirma que el adjunto se subió a S3 (HEAD del objeto) y devuelve una presigned GET URL (1h). Llamar después de haber subido los bytes a la upload_url de init_upload."),
		mcp.WithString("attachment_id", mcp.Description("UUID del attachment (de init_upload)"), mcp.Required()),
	)
}

func toolAttachmentGetURL() mcp.Tool {
	return mcp.NewTool("domain_attachment_get_url",
		mcp.WithDescription("Devuelve una presigned GET URL (1h) para descargar un adjunto ya confirmado. El server no proxya bytes."),
		mcp.WithString("attachment_id", mcp.Description("UUID del attachment"), mcp.Required()),
	)
}

func toolAttachmentList() mcp.Tool {
	return mcp.NewTool("domain_attachment_list",
		mcp.WithDescription("Lista los adjuntos de una entidad (metadata, sin URLs). Para descargar uno, usar domain_attachment_get_url con su id."),
		mcp.WithString("entity_type", mcp.Description(attachmentEntityDesc), mcp.Required()),
		mcp.WithString("entity_id", mcp.Description("UUID de la entidad dueña"), mcp.Required()),
	)
}

func toolAttachmentDelete() mcp.Tool {
	return mcp.NewTool("domain_attachment_delete",
		mcp.WithDescription("Borra un adjunto: elimina la fila en file_attachments y el objeto en S3. Idempotente sobre el objeto S3 (si ya no existe, no falla)."),
		mcp.WithString("attachment_id", mcp.Description("UUID del attachment a borrar"), mcp.Required()),
	)
}

// attachmentError traduce los errores de negocio del servicio a un mensaje de
// tool claro. Devuelve "" si err no es uno de los conocidos.
func attachmentError(err error) string {
	switch {
	case errors.Is(err, attachmentsvc.ErrNotFound):
		return "attachment no encontrado"
	case errors.Is(err, attachmentsvc.ErrTooLarge):
		return "archivo demasiado grande (máx 10MB)"
	case errors.Is(err, attachmentsvc.ErrTypeNotAllowed):
		return "MIME type no permitido (image/*, application/pdf, text/markdown, text/plain)"
	case errors.Is(err, attachmentsvc.ErrInvalidEntity):
		return "entity_type inválido o entidad inexistente"
	default:
		return ""
	}
}

func (h *attachmentHandlers) requireDeps() error {
	if h.principal == nil {
		return fmt.Errorf("no authenticated principal")
	}
	if h.attachments == nil {
		return fmt.Errorf("attachment service not configured (falta S3/MinIO)")
	}
	return nil
}

func (h *attachmentHandlers) toolErr(err error, prefix string) *mcp.CallToolResult {
	if msg := attachmentError(err); msg != "" {
		return mcp.NewToolResultError(msg)
	}
	return mcp.NewToolResultError(fmt.Sprintf("%s: %v", prefix, err))
}

func (h *attachmentHandlers) handleAttachmentInitUpload(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := req.GetArguments()
	entityType, _ := args["entity_type"].(string)
	entityID, _ := args["entity_id"].(string)
	filename, _ := args["filename"].(string)
	mimeType, _ := args["mime_type"].(string)
	sizeF, _ := args["size_bytes"].(float64)
	if entityType == "" || entityID == "" || filename == "" || mimeType == "" {
		return mcp.NewToolResultError("entity_type, entity_id, filename y mime_type son requeridos"), nil
	}
	res, err := h.attachments.InitUpload(ctx, entityType, entityID, filename, mimeType, h.principal.UserID, int64(sizeF))
	if err != nil {
		return h.toolErr(err, "init_upload failed"), nil
	}
	return toolResultJSON(res)
}

func (h *attachmentHandlers) handleAttachmentConfirm(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	id, err := uuid.Parse(req.GetArguments()["attachment_id"].(string))
	if err != nil {
		return mcp.NewToolResultError("attachment_id inválido"), nil
	}
	res, err := h.attachments.ConfirmUpload(ctx, id)
	if err != nil {
		return h.toolErr(err, "confirm failed"), nil
	}
	return toolResultJSON(res)
}

func (h *attachmentHandlers) handleAttachmentGetURL(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	id, err := uuid.Parse(req.GetArguments()["attachment_id"].(string))
	if err != nil {
		return mcp.NewToolResultError("attachment_id inválido"), nil
	}
	res, err := h.attachments.GetDownloadURL(ctx, id)
	if err != nil {
		return h.toolErr(err, "get_url failed"), nil
	}
	return toolResultJSON(res)
}

func (h *attachmentHandlers) handleAttachmentList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	args := req.GetArguments()
	entityType, _ := args["entity_type"].(string)
	entityID, _ := args["entity_id"].(string)
	if entityType == "" || entityID == "" {
		return mcp.NewToolResultError("entity_type y entity_id son requeridos"), nil
	}
	list, err := h.attachments.ListByEntity(ctx, entityType, entityID)
	if err != nil {
		return h.toolErr(err, "list failed"), nil
	}
	return toolResultJSON(list)
}

func (h *attachmentHandlers) handleAttachmentDelete(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if err := h.requireDeps(); err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	id, err := uuid.Parse(req.GetArguments()["attachment_id"].(string))
	if err != nil {
		return mcp.NewToolResultError("attachment_id inválido"), nil
	}
	if err := h.attachments.Delete(ctx, id); err != nil {
		return h.toolErr(err, "delete failed"), nil
	}
	return toolResultJSON(map[string]any{"deleted": true, "attachment_id": id.String()})
}
