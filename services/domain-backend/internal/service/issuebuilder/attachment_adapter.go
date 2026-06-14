package issuebuilder

import (
	"context"

	"github.com/google/uuid"

	"nunezlagos/domain/internal/service/attachment"
)

// AttachmentServiceAdapter envuelve internal/service/attachment.Service
// para satisfacer la interface issuebuilder.AttachmentService sin import
// cycle. Wire en main.go con:
//
//	issuebuilder.Service{Attachments: &issuebuilder.AttachmentServiceAdapter{Inner: attService}}
type AttachmentServiceAdapter struct {
	Inner *attachment.Service
}

// InitUpload satisface issuebuilder.AttachmentService.
func (a *AttachmentServiceAdapter) InitUpload(ctx context.Context, entityType, entityIDStr, filename, mimeType, createdBy string, size int64) (*AttachmentInitResult, error) {
	res, err := a.Inner.InitUpload(ctx, entityType, entityIDStr, filename, mimeType, createdBy, size)
	if err != nil {
		return nil, err
	}
	return &AttachmentInitResult{
		AttachmentID: res.Attachment.ID,
		UploadURL:    res.UploadURL,
		Filename:     res.Attachment.Filename,
	}, nil
}

// PromoteEntity satisface issuebuilder.AttachmentService.
func (a *AttachmentServiceAdapter) PromoteEntity(ctx context.Context, fromKind, toKind string, fromID, toID uuid.UUID) (int, error) {
	return a.Inner.PromoteEntity(ctx, fromKind, toKind, fromID, toID)
}
