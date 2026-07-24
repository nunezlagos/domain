// Tests unitarios de los tool builders + handlers de attachments (DOMAINSERV-79
// H1). Sin DB ni S3: se inyecta un fake del attachmentService por la interface
// consumer-defined.
package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/mcp"

	"nunezlagos/domain/internal/auth/apikey"
	attachmentsvc "nunezlagos/domain/internal/service/attachment"
)

type fakeAttachmentSvc struct {
	initRes    *attachmentsvc.InitUploadResult
	confirmRes *attachmentsvc.ConfirmResult
	list       []attachmentsvc.Attachment
	err        error
	deleteErr  error
	deletedID  uuid.UUID
}

func (f *fakeAttachmentSvc) InitUpload(ctx context.Context, entityType, entityID, filename, mimeType, createdBy string, size int64) (*attachmentsvc.InitUploadResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.initRes, nil
}
func (f *fakeAttachmentSvc) ConfirmUpload(ctx context.Context, id uuid.UUID) (*attachmentsvc.ConfirmResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.confirmRes, nil
}
func (f *fakeAttachmentSvc) GetDownloadURL(ctx context.Context, id uuid.UUID) (*attachmentsvc.ConfirmResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.confirmRes, nil
}
func (f *fakeAttachmentSvc) ListByEntity(ctx context.Context, entityType, entityID string) ([]attachmentsvc.Attachment, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.list, nil
}
func (f *fakeAttachmentSvc) Delete(ctx context.Context, id uuid.UUID) error {
	f.deletedID = id
	return f.deleteErr
}

func attHandlers(svc attachmentService) *attachmentHandlers {
	return &attachmentHandlers{
		attachments: svc,
		principal: &apikey.Principal{
			OrganizationID: "11111111-1111-1111-1111-111111111111",
			UserID:         "22222222-2222-2222-2222-222222222222",
		},
	}
}

func reqWith(args map[string]any) mcp.CallToolRequest {
	var req mcp.CallToolRequest
	req.Params.Arguments = args
	return req
}

func resultText(res *mcp.CallToolResult) string {
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

func TestToolAttachment_Names(t *testing.T) {
	cases := map[string]mcp.Tool{
		"domain_attachment_init_upload": toolAttachmentInitUpload(),
		"domain_attachment_confirm":     toolAttachmentConfirm(),
		"domain_attachment_get_url":     toolAttachmentGetURL(),
		"domain_attachment_list":        toolAttachmentList(),
		"domain_attachment_delete":      toolAttachmentDelete(),
	}
	for want, tool := range cases {
		if tool.Name != want {
			t.Errorf("tool name=%s want=%s", tool.Name, want)
		}
		if tool.Description == "" {
			t.Errorf("tool %s: description vacío", want)
		}
	}
}

func TestHandleAttachmentInitUpload_SinPrincipal_Error(t *testing.T) {
	h := &attachmentHandlers{attachments: &fakeAttachmentSvc{}}
	res, err := h.handleAttachmentInitUpload(context.Background(), reqWith(nil))
	if err != nil {
		t.Fatalf("err inesperado: %v", err)
	}
	if !res.IsError {
		t.Fatalf("esperaba error sin principal")
	}
}

func TestHandleAttachmentInitUpload_SinService_Error(t *testing.T) {
	h := &attachmentHandlers{principal: &apikey.Principal{OrganizationID: "x", UserID: "y"}}
	res, _ := h.handleAttachmentInitUpload(context.Background(), reqWith(nil))
	if !res.IsError {
		t.Fatalf("esperaba error sin service (attachments nil)")
	}
}

func TestHandleAttachmentInitUpload_Happy(t *testing.T) {
	h := attHandlers(&fakeAttachmentSvc{initRes: &attachmentsvc.InitUploadResult{
		Attachment: attachmentsvc.Attachment{Filename: "doc.pdf"},
		UploadURL:  "https://minio.local/put/doc.pdf",
	}})
	res, _ := h.handleAttachmentInitUpload(context.Background(), reqWith(map[string]any{
		"entity_type": "requirement",
		"entity_id":   "33333333-3333-3333-3333-333333333333",
		"filename":    "doc.pdf",
		"mime_type":   "application/pdf",
		"size_bytes":  float64(1024),
	}))
	if res.IsError {
		t.Fatalf("esperaba éxito, got error: %s", resultText(res))
	}
	if !strings.Contains(resultText(res), "https://minio.local/put/doc.pdf") {
		t.Errorf("el resultado debe incluir la upload URL, got: %s", resultText(res))
	}
}

func TestHandleAttachmentInitUpload_MissingArgs_Error(t *testing.T) {
	h := attHandlers(&fakeAttachmentSvc{})
	res, _ := h.handleAttachmentInitUpload(context.Background(), reqWith(map[string]any{
		"entity_type": "requirement",
		"entity_id":   "33333333-3333-3333-3333-333333333333",
		// falta filename y mime_type
	}))
	if !res.IsError {
		t.Fatalf("esperaba error por args faltantes")
	}
}

func TestHandleAttachmentInitUpload_ErrorMapping(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{attachmentsvc.ErrTooLarge, "demasiado grande"},
		{attachmentsvc.ErrTypeNotAllowed, "MIME"},
		{attachmentsvc.ErrInvalidEntity, "entity_type"},
	}
	for _, c := range cases {
		h := attHandlers(&fakeAttachmentSvc{err: c.err})
		res, _ := h.handleAttachmentInitUpload(context.Background(), reqWith(map[string]any{
			"entity_type": "requirement", "entity_id": "33333333-3333-3333-3333-333333333333",
			"filename": "x.pdf", "mime_type": "application/pdf", "size_bytes": float64(1),
		}))
		if !res.IsError {
			t.Fatalf("esperaba error para %v", c.err)
		}
		if !strings.Contains(resultText(res), c.want) {
			t.Errorf("mensaje para %v debe contener %q, got: %s", c.err, c.want, resultText(res))
		}
	}
}

func TestHandleAttachmentConfirm_InvalidUUID_Error(t *testing.T) {
	h := attHandlers(&fakeAttachmentSvc{})
	res, _ := h.handleAttachmentConfirm(context.Background(), reqWith(map[string]any{"attachment_id": "no-uuid"}))
	if !res.IsError {
		t.Fatalf("esperaba error por UUID inválido")
	}
}

func TestHandleAttachmentList_Happy(t *testing.T) {
	h := attHandlers(&fakeAttachmentSvc{list: []attachmentsvc.Attachment{{Filename: "a.png"}, {Filename: "b.pdf"}}})
	res, _ := h.handleAttachmentList(context.Background(), reqWith(map[string]any{
		"entity_type": "user_story", "entity_id": "44444444-4444-4444-4444-444444444444",
	}))
	if res.IsError {
		t.Fatalf("esperaba éxito, got: %s", resultText(res))
	}
	if !strings.Contains(resultText(res), "a.png") || !strings.Contains(resultText(res), "b.pdf") {
		t.Errorf("el listado debe incluir ambos filenames, got: %s", resultText(res))
	}
}

func TestHandleAttachmentDelete_Happy(t *testing.T) {
	fake := &fakeAttachmentSvc{}
	h := attHandlers(fake)
	id := "55555555-5555-5555-5555-555555555555"
	res, _ := h.handleAttachmentDelete(context.Background(), reqWith(map[string]any{"attachment_id": id}))
	if res.IsError {
		t.Fatalf("esperaba éxito, got: %s", resultText(res))
	}
	if !strings.Contains(resultText(res), "\"deleted\": true") {
		t.Errorf("resultado debe indicar deleted:true, got: %s", resultText(res))
	}
	if fake.deletedID.String() != id {
		t.Errorf("Delete recibió id=%s want=%s", fake.deletedID, id)
	}
}
