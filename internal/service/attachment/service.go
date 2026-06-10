// Package attachment — HU-04.6 S3 file attachments for entities.
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
	s3client "nunezlagos/domain/internal/storage/s3"
)

const maxFileSize int64 = 10 * 1024 * 1024 // 10MB

var allowedMIMEPrefixes = []string{
	"image/", "application/pdf", "text/markdown", "text/plain",
}

var (
	ErrNotFound     = errors.New("attachment not found")
	ErrTooLarge     = errors.New("file too large: max 10MB")
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

// Service manages file attachments with S3 presigned URLs.
type Service struct {
	Pool  *pgxpool.Pool
	S3    *s3client.Client
	Audit *audit.PGRecorder
}

// InitUpload creates attachment record + presigned PUT URL.
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

	var a Attachment
	err = s.Pool.QueryRow(ctx,
		`INSERT INTO file_attachments (entity_type, entity_id, filename, s3_key, size_bytes, mime_type, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 RETURNING id, entity_type, entity_id, filename, s3_key, size_bytes, mime_type, created_by, created_at`,
		entityType, entityID, filename, s3Key, size, mimeType, cb,
	).Scan(&a.ID, &a.EntityType, &a.EntityID, &a.S3Key, &a.SizeBytes, &a.MimeType, &a.CreatedBy, &a.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert attachment: %w", err)
	}

	uploadURL, err := s.S3.GenerateUploadURL(ctx, s3Key)
	if err != nil {
		return nil, fmt.Errorf("generate upload url: %w", err)
	}

	return &InitUploadResult{Attachment: a, UploadURL: uploadURL}, nil
}

// ConfirmUpload verifies the object exists in S3 and returns download URL.
func (s *Service) ConfirmUpload(ctx context.Context, attachmentID uuid.UUID) (*ConfirmResult, error) {
	var a Attachment
	err := s.Pool.QueryRow(ctx,
		`SELECT id, entity_type, entity_id, filename, s3_key, size_bytes, mime_type, created_by, created_at
		 FROM file_attachments WHERE id = $1`, attachmentID,
	).Scan(&a.ID, &a.EntityType, &a.EntityID, &a.Filename, &a.S3Key, &a.SizeBytes, &a.MimeType, &a.CreatedBy, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get attachment: %w", err)
	}

	exists, err := s.S3.ConfirmObject(ctx, a.S3Key)
	if err != nil {
		return nil, fmt.Errorf("confirm s3: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("object not found in s3 after upload")
	}

	dlURL, err := s.S3.GenerateDownloadURL(ctx, a.S3Key)
	if err != nil {
		return nil, fmt.Errorf("generate download url: %w", err)
	}

	return &ConfirmResult{Attachment: a, DownloadURL: dlURL}, nil
}

// GetDownloadURL returns presigned GET URL for an attachment.
func (s *Service) GetDownloadURL(ctx context.Context, attachmentID uuid.UUID) (*ConfirmResult, error) {
	var a Attachment
	err := s.Pool.QueryRow(ctx,
		`SELECT id, entity_type, entity_id, filename, s3_key, size_bytes, mime_type, created_by, created_at
		 FROM file_attachments WHERE id = $1`, attachmentID,
	).Scan(&a.ID, &a.EntityType, &a.EntityID, &a.Filename, &a.S3Key, &a.SizeBytes, &a.MimeType, &a.CreatedBy, &a.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get attachment: %w", err)
	}

	dlURL, err := s.S3.GenerateDownloadURL(ctx, a.S3Key)
	if err != nil {
		return nil, fmt.Errorf("generate download url: %w", err)
	}

	return &ConfirmResult{Attachment: a, DownloadURL: dlURL}, nil
}

// ListByEntity returns attachments for a given entity.
func (s *Service) ListByEntity(ctx context.Context, entityType, entityIDStr string) ([]Attachment, error) {
	entityID, err := uuid.Parse(entityIDStr)
	if err != nil {
		return nil, ErrInvalidEntity
	}
	rows, err := s.Pool.Query(ctx,
		`SELECT id, entity_type, entity_id, filename, s3_key, size_bytes, mime_type, created_by, created_at
		 FROM file_attachments WHERE entity_type = $1 AND entity_id = $2 ORDER BY created_at DESC`,
		entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("list attachments: %w", err)
	}
	defer rows.Close()

	var out []Attachment
	for rows.Next() {
		var a Attachment
		if err := rows.Scan(&a.ID, &a.EntityType, &a.EntityID, &a.Filename, &a.S3Key, &a.SizeBytes, &a.MimeType, &a.CreatedBy, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		out = append(out, a)
	}
	return out, nil
}

// Delete removes attachment record and S3 object.
func (s *Service) Delete(ctx context.Context, attachmentID uuid.UUID) error {
	var s3Key string
	err := s.Pool.QueryRow(ctx, `DELETE FROM file_attachments WHERE id = $1 RETURNING s3_key`, attachmentID).Scan(&s3Key)
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

// CleanupOrphans removes attachments and S3 objects for deleted entities.
func (s *Service) CleanupOrphans(ctx context.Context) (int, error) {
	rows, err := s.Pool.Query(ctx, `
		DELETE FROM file_attachments fa
		WHERE NOT EXISTS (SELECT 1 FROM user_stories WHERE id = fa.entity_id AND entity_type = 'user_story')
		  AND NOT EXISTS (SELECT 1 FROM requirements WHERE id = fa.entity_id AND entity_type = 'requirement')
		RETURNING s3_key
	`)
	if err != nil {
		return 0, fmt.Errorf("cleanup orphans: %w", err)
	}
	defer rows.Close()

	var count int
	for rows.Next() {
		var s3Key string
		if err := rows.Scan(&s3Key); err != nil {
			continue
		}
		_ = s.S3.DeleteObject(ctx, s3Key)
		count++
	}
	return count, nil
}

// --- helpers ---

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
		table = "user_stories"
	case "requirement":
		table = "requirements"
	case "hu_draft":
		table = "hu_drafts"
	case "intake_payload":
		table = "intake_payloads"
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

// PromoteEntity reasigna todos los attachments de (fromKind, fromID) a
// (toKind, toID). Usado cuando un draft (HU-04.7) se commit como user_story
// real, o un intake_payload se transforma en HU/REQ. Idempotente.
func (s *Service) PromoteEntity(ctx context.Context, fromKind, toKind string, fromID, toID uuid.UUID) (int, error) {
	if err := s.requireEntity(ctx, toKind, toID); err != nil {
		return 0, fmt.Errorf("target entity: %w", err)
	}
	tag, err := s.Pool.Exec(ctx, `
		UPDATE file_attachments
		SET entity_type = $1, entity_id = $2
		WHERE entity_type = $3 AND entity_id = $4`,
		toKind, toID, fromKind, fromID,
	)
	if err != nil {
		return 0, fmt.Errorf("promote: %w", err)
	}
	return int(tag.RowsAffected()), nil
}
