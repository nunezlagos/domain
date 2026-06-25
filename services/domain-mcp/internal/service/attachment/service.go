package attachment

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nunezlagos/domain/internal/audit"
	"nunezlagos/domain/internal/service/attachment/attachmentdb"
	s3client "nunezlagos/domain/internal/storage/s3"
	"nunezlagos/domain/internal/store/txctx"
)

const maxFileSize int64 = 10 * 1024 * 1024 // 10MB

var allowedMIMEPrefixes = []string{
	"image/", "application/pdf", "text/markdown", "text/plain",
}

var (
	ErrNotFound       = errors.New("attachment not found")
	ErrTooLarge       = errors.New("file too large: max 10MB")
	ErrTypeNotAllowed = errors.New("file type not allowed")
	ErrInvalidEntity  = errors.New("invalid entity reference")
)

type Attachment struct {
	ID         uuid.UUID `json:"id"`
	EntityType string    `json:"entity_type"`
	EntityID   uuid.UUID `json:"entity_id"`
	Filename   string    `json:"filename"`
	S3Key      string    `json:"s3_key,omitempty"`
	SizeBytes  int64     `json:"size_bytes"`
	MimeType   string    `json:"mime_type"`
	CreatedBy  *string   `json:"created_by,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type InitUploadResult struct {
	Attachment Attachment `json:"attachment"`
	UploadURL  string     `json:"upload_url"`
}

type ConfirmResult struct {
	Attachment  Attachment `json:"attachment"`
	DownloadURL string     `json:"download_url"`
}

type Service struct {
	Pool  *pgxpool.Pool
	S3    *s3client.Client
	Audit *audit.PGRecorder
}

func (s *Service) q(ctx context.Context) *attachmentdb.Queries {
	if tx := txctx.TxFromContext(ctx); tx != nil {
		return attachmentdb.New(tx)
	}
	return attachmentdb.New(s.Pool)
}

func toAttachment(fa attachmentdb.FileAttachment) Attachment {
	return Attachment{
		ID:         fa.ID,
		EntityType: fa.EntityType,
		EntityID:   fa.EntityID,
		Filename:   fa.Filename,
		S3Key:      fa.S3Key,
		SizeBytes:  fa.SizeBytes,
		MimeType:   fa.MimeType,
		CreatedBy:  fa.CreatedBy,
		CreatedAt:  fa.CreatedAt,
	}
}

func (s *Service) InitUpload(ctx context.Context, entityType, entityIDStr, filename, mimeType, createdBy string, size int64) (*InitUploadResult, error) {
	if err := validateFile(size, mimeType); err != nil {
		return nil, err
	}
	entityID, err := uuid.Parse(entityIDStr)
	if err != nil {
		return nil, ErrInvalidEntity
	}
	if err := s.requireEntity(ctx, entityType, entityID); err != nil {
		return nil, err
	}

	s3Key := fmt.Sprintf("attachments/%s/%s/%s", entityType, entityID, filename)

	var cb *string
	if createdBy != "" {
		cb = &createdBy
	}

	fa, err := s.q(ctx).InsertAttachment(ctx, attachmentdb.InsertAttachmentParams{
		EntityType: entityType,
		EntityID:   entityID,
		Filename:   filename,
		S3Key:      s3Key,
		SizeBytes:  size,
		MimeType:   mimeType,
		CreatedBy:  cb,
	})
	if err != nil {
		return nil, fmt.Errorf("insert attachment: %w", err)
	}

	uploadURL, err := s.S3.GenerateUploadURL(ctx, s3Key)
	if err != nil {
		return nil, fmt.Errorf("generate upload url: %w", err)
	}

	return &InitUploadResult{Attachment: toAttachment(fa), UploadURL: uploadURL}, nil
}

func (s *Service) ConfirmUpload(ctx context.Context, attachmentID uuid.UUID) (*ConfirmResult, error) {
	fa, err := s.q(ctx).GetAttachment(ctx, attachmentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get attachment: %w", err)
	}

	exists, err := s.S3.ConfirmObject(ctx, fa.S3Key)
	if err != nil {
		return nil, fmt.Errorf("confirm s3: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("object not found in s3 after upload")
	}

	dlURL, err := s.S3.GenerateDownloadURL(ctx, fa.S3Key)
	if err != nil {
		return nil, fmt.Errorf("generate download url: %w", err)
	}

	return &ConfirmResult{Attachment: toAttachment(fa), DownloadURL: dlURL}, nil
}

func (s *Service) GetDownloadURL(ctx context.Context, attachmentID uuid.UUID) (*ConfirmResult, error) {
	fa, err := s.q(ctx).GetAttachment(ctx, attachmentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get attachment: %w", err)
	}

	dlURL, err := s.S3.GenerateDownloadURL(ctx, fa.S3Key)
	if err != nil {
		return nil, fmt.Errorf("generate download url: %w", err)
	}

	return &ConfirmResult{Attachment: toAttachment(fa), DownloadURL: dlURL}, nil
}

func (s *Service) ListByEntity(ctx context.Context, entityType, entityIDStr string) ([]Attachment, error) {
	entityID, err := uuid.Parse(entityIDStr)
	if err != nil {
		return nil, ErrInvalidEntity
	}
	items, err := s.q(ctx).ListByEntity(ctx, attachmentdb.ListByEntityParams{
		EntityType: entityType,
		EntityID:   entityID,
	})
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}

	out := make([]Attachment, len(items))
	for i, fa := range items {
		out[i] = toAttachment(fa)
	}
	return out, nil
}

func (s *Service) Delete(ctx context.Context, attachmentID uuid.UUID) error {
	s3Key, err := s.q(ctx).DeleteAttachment(ctx, attachmentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("delete attachment: %w", err)
	}
	if err := s.S3.DeleteObject(ctx, s3Key); err != nil {
		return fmt.Errorf("delete s3 object: %w", err)
	}
	return nil
}

func (s *Service) CleanupOrphans(ctx context.Context) (int, error) {
	keys, err := s.q(ctx).CleanupOrphans(ctx)
	if err != nil {
		return 0, fmt.Errorf("cleanup orphans: %w", err)
	}

	for _, s3Key := range keys {
		_ = s.S3.DeleteObject(ctx, s3Key)
	}
	return len(keys), nil
}

func validateFile(size int64, mimeType string) error {
	if size > maxFileSize {
		return ErrTooLarge
	}
	allowed := false
	for _, prefix := range allowedMIMEPrefixes {
		if strings.HasPrefix(mimeType, prefix) {
			allowed = true
			break
		}
	}
	if !allowed {
		return ErrTypeNotAllowed
	}
	return nil
}

func (s *Service) requireEntity(ctx context.Context, entityType string, entityID uuid.UUID) error {
	var table string
	switch entityType {
	case "user_story":
		table = "issues"
	case "requirement":
		table = "sdd_requirements"
	case "hu_draft":
		table = "issue_drafts"
	case "intake_payload":
		table = "issue_intake_payloads"
	default:
		return ErrInvalidEntity
	}
	var exists bool
	err := s.Pool.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s WHERE id = $1)`, table), entityID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check entity: %w", err)
	}
	if !exists {
		return ErrInvalidEntity
	}
	return nil
}

func (s *Service) PromoteEntity(ctx context.Context, fromKind, toKind string, fromID, toID uuid.UUID) (int, error) {
	if err := s.requireEntity(ctx, toKind, toID); err != nil {
		return 0, fmt.Errorf("target entity: %w", err)
	}
	n, err := s.q(ctx).PromoteEntity(ctx, attachmentdb.PromoteEntityParams{
		ToType:   toKind,
		ToID:     toID,
		FromType: fromKind,
		FromID:   fromID,
	})
	if err != nil {
		return 0, fmt.Errorf("promote: %w", err)
	}
	return int(n), nil
}
