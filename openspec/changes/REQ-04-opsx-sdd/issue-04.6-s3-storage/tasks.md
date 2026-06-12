# Tasks: issue-04.6-s3-storage

## Backend

- [x] **s3-001**: Migration `XXXXXX_create_file_attachments.up.sql`
- [x] **s3-002**: `internal/storage/s3/client.go` — NewS3Client con endpoint configurable + session
- [x] **s3-003**: `internal/storage/s3/upload.go` — GenerateUploadURL (presigned PUT, 15min expiry)
- [x] **s3-004**: `internal/storage/s3/download.go` — GenerateDownloadURL (presigned GET, 1h expiry)
- [x] **s3-005**: `internal/storage/s3/confirm.go` — ConfirmUpload (HEAD object exists + metadata)
- [x] **s3-006**: `internal/storage/s3/cleanup.go` — CleanupOrphans (list + delete S3 + DB)
- [x] **s3-007**: `internal/service/attachment/service.go` — AttachmentService orquestando S3 + DB + validación
- [x] **s3-008**: Validación tamaño (max 10MB) y tipo MIME (whitelist)
- [x] **s3-009**: Handler POST /api/v1/attachments (init upload)
- [x] **s3-010**: Handler POST /api/v1/attachments/:id/confirm
- [x] **s3-011**: Handler GET /api/v1/attachments/:id/download
- [x] **s3-012**: Handler GET /api/v1/attachments (list por entity)
- [x] **s3-013**: Handler DELETE /api/v1/attachments/:id
- [x] **s3-014**: Wiring en main.go + api.go
- [x] **s3-015**: Cron cleanupOrphans (diario, solo leader)

## Tests

- [x] Test integración MinIO: upload presigned → confirm → download
- [x] Test validación: archivo >10MB → rechazado
- [x] Test validación: tipo no permitido → rechazado
- [x] Test cleanup: attach a HU → borrar HU → cleanup elimina S3 + DB
- [x] Test signed URL expira → 403 de S3
- [x] Sabotaje: entity_id UUID inválido → 400 no panic
- [x] Sabotaje: MinIO caído → error claro, no panic

## Cierre

- [x] Verificación manual con MinIO local
- [x] Suite verde
