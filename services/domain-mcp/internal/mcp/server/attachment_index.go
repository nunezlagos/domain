package mcpserver

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"

	knowsvc "nunezlagos/domain/internal/service/knowledge"
)

// handleAttachmentIndex (DOMAINSERV-79 H3): indexa el TEXTO de un adjunto a
// knowledge_docs para que sea buscable por domain_knowledge_search. El CLIENTE
// provee el texto ya extraído (MD/txt/PDF): respeta el invariante "el server NO
// proxya bytes" — no baja el objeto de S3 ni agrega deps de extracción. Compone
// Knowledge.Save + Projects.GetBySlug (patrón de handleKnowledgeSave) sin acoplar
// el servicio de attachments.
func (d *Deps) handleAttachmentIndex(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	if d.Principal == nil || d.Knowledge == nil {
		return mcp.NewToolResultError("knowledge service no configurado"), nil
	}
	args := req.GetArguments()
	attIDStr, _ := args["attachment_id"].(string)
	slug, _ := args["project_slug"].(string)
	text, _ := args["text"].(string)
	title, _ := args["title"].(string)
	if attIDStr == "" || slug == "" || text == "" {
		return mcp.NewToolResultError("attachment_id, project_slug y text son requeridos"), nil
	}
	attID, err := uuid.Parse(attIDStr)
	if err != nil {
		return mcp.NewToolResultError("attachment_id inválido"), nil
	}
	if title == "" {
		title = "adjunto " + attIDStr
	}
	orgID, _ := uuid.Parse(d.Principal.OrganizationID)
	userID, _ := uuid.Parse(d.Principal.UserID)
	proj, err := d.Projects.GetBySlug(ctx, orgID, slug)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("project '%s' not found", slug)), nil
	}
	doc, chunks, err := d.Knowledge.Save(ctx, knowsvc.SaveInput{
		OrganizationID: orgID, ProjectID: proj.ID, CreatedBy: &userID,
		Title: title, Body: text, Source: "attachment",
		Tags:     []string{"attachment"},
		Metadata: map[string]any{"attachment_id": attID.String()},
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("index: %v", err)), nil
	}
	return toolResultJSON(map[string]any{
		"id":            doc.ID.String(),
		"attachment_id": attID.String(),
		"chunks_count":  len(chunks),
	})
}

func toolAttachmentIndex() mcp.Tool {
	return mcp.NewTool("domain_attachment_index",
		mcp.WithDescription("DOMAINSERV-79 H3: indexa el TEXTO de un adjunto (que el CLIENTE ya extrajo de MD/txt/PDF) a knowledge_docs para que sea buscable por domain_knowledge_search. El server NO baja el objeto de S3 (no proxya bytes): el cliente pasa el texto."),
		mcp.WithString("attachment_id",
			mcp.Description("UUID del adjunto (de domain_attachment_init_upload/list) — se guarda como link en metadata."),
			mcp.Required(),
		),
		mcp.WithString("project_slug",
			mcp.Description("Slug del project donde indexar el knowledge_doc."),
			mcp.Required(),
		),
		mcp.WithString("text",
			mcp.Description("Texto extraído del adjunto (el cliente lo extrae; el server lo chunkea + embebe)."),
			mcp.Required(),
		),
		mcp.WithString("title",
			mcp.Description("Título del doc (opcional; default 'adjunto <id>'). Usá el filename."),
		),
	)
}
