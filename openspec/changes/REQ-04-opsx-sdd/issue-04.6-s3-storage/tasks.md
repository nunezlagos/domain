# Tasks: issue-04.6-s3-storage

## Backend

- [ ] **s3-001**: Migration `XXXXXX_create_file_attachments.up.sql`
- [ ] **s3-002**: `internal/storage/s3/client.go` — NewS3Client con endpoint configurable + session
- [ ] **s3-003**: `internal/storage/s3/upload.go` — GenerateUploadURL (presigned PUT, 15min expiry)
- [ ] **s3-004**: `internal/storage/s3/download.go` — GenerateDownloadURL (presigned GET, 1h expiry)
- [ ] **s3-005**: `internal/storage/s3/confirm.go` — ConfirmUpload (HEAD object exists + metadata)
- [ ] **s3-006**: `internal/storage/s3/cleanup.go` — CleanupOrphans (list + delete S3 + DB)
- [ ] **s3-007**: `internal/service/attachment/service.go` — AttachmentService orquestando S3 + DB + validación
- [ ] **s3-008**: Validación tamaño (max 10MB) y tipo MIME (whitelist)
- [ ] **s3-009**: Handler POST /api/v1/attachments (init upload)
- [ ] **s3-010**: Handler POST /api/v1/attachments/:id/confirm
- [ ] **s3-011**: Handler GET /api/v1/attachments/:id/download
- [ ] **s3-012**: Handler GET /api/v1/attachments (list por entity)
- [ ] **s3-013**: Handler DELETE /api/v1/attachments/:id
- [ ] **s3-014**: Wiring en main.go + api.go
- [ ] **s3-015**: Cron cleanupOrphans (diario, solo leader)

## Tests

- [ ] Test integración MinIO: upload presigned → confirm → download
- [ ] Test validación: archivo >10MB → rechazado
- [ ] Test validación: tipo no permitido → rechazado
- [ ] Test cleanup: attach a HU → borrar HU → cleanup elimina S3 + DB
- [ ] Test signed URL expira → 403 de S3
- [ ] Sabotaje: entity_id UUID inválido → 400 no panic
- [ ] Sabotaje: MinIO caído → error claro, no panic

## Cierre

- [ ] Verificación manual con MinIO local
- [ ] Suite verde
