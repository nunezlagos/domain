# Design: issue-04.6-s3-storage

## Decisión arquitectónica

**Backend genera presigned URLs; cliente sube/baja directo a S3.** El backend nunca toca el binary del archivo, solo genera URLs firmadas y persiste metadatos en `file_attachments`.

**AWS SDK v2** con endpoint configurable para soportar MinIO / R2 / DO Spaces.

```
file_attachments
├── id              UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── entity_type     VARCHAR(50) NOT NULL    -- 'user_story' | 'requirement' | 'knowledge_doc'
├── entity_id       UUID NOT NULL
├── filename        VARCHAR(255) NOT NULL
├── s3_key          TEXT NOT NULL UNIQUE     -- "{entity_type}/{entity_id}/{filename}"
├── size_bytes      BIGINT NOT NULL
├── mime_type       VARCHAR(127) NOT NULL
├── created_by      VARCHAR(255)
├── created_at      TIMESTAMPTZ NOT NULL DEFAULT now()

INDEX (entity_type, entity_id)
```

**S3 key convention:** `attachments/{entity_type}/{entity_id}/{filename}`

**Allowed MIME types:** image/png, image/jpeg, image/gif, image/webp, image/svg+xml, application/pdf, text/markdown, text/plain

**Max file size:** 10MB (reject antes de subir a S3)

## Alternativas descartadas

| Alternativa | Motivo |
|-------------|--------|
| Upload proxy (backend recibe binary) | Consume memoria/CPU innecesario; S3 presigned es más eficiente |
| Base64 en JSON body | Ineficiente (+33% tamaño); no escala |
| Filesystem local | No escala horizontalmente; S3 es el estándar cloud |
| Multipart upload | Overkill para <10MB; añade complejidad |

## Diagrama

```
Client                    Backend                        S3
  │                         │                            │
  │ POST /attachments       │                            │
  ├────────────────────────>│                            │
  │                         │── Validate size & type     │
  │                         │── Generate s3_key          │
  │                         │── Persist file_attachments │
  │                         │── Generate presigned PUT   │
  │       {upload_url, id}  │                            │
  │<────────────────────────┤                            │
  │ PUT {upload_url}        │                            │
  │ [binary]                │                            │
  ├─────────────────────────────────────────────────────>│
  │                         │                            │
  │ POST /attachments/:id/confirm                         │
  ├────────────────────────>│                            │
  │                         │── HEAD s3 verify           │
  │       {download_url}    │                            │
  │<────────────────────────┤                            │
```

**Download flow (GET /attachments/:id/download):**
1. Lookup file_attachments row
2. Generate presigned GET URL (1h)
3. Return `{download_url, filename, mime}`

**Cleanup (cron):**
```sql
DELETE FROM file_attachments fa
WHERE NOT EXISTS (SELECT 1 FROM issues WHERE id = fa.entity_id AND entity_type = 'user_story')
  AND NOT EXISTS (SELECT 1 FROM requirements WHERE id = fa.entity_id AND entity_type = 'requirement')
  AND NOT EXISTS (SELECT 1 FROM knowledge_docs WHERE id = fa.entity_id AND entity_type = 'knowledge_doc');
```
(Previo: delete de S3 objects en batch)

## TDD plan

1. Red: Test InitS3 con endpoint custom → cliente configurado
2. Green: NewS3Client con MinIO container (testcontainers)
3. Red: Test presigned PUT URL generada → subir archivo
4. Green: GenerateUploadURL
5. Red: Test ConfirmUpload con HEAD → verifica exists
6. Green: ConfirmUpload
7. Red: Test download presigned URL → GET archivo
8. Green: GenerateDownloadURL
9. Red: Test file too large reject
10. Green: ValidateFile
11. Red: Test cleanup orphans
12. Green: CleanupOrphans
13. Sabotaje: entity_id que no existe → 404 no panic

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| S3 client timeouts | Configurable connect/request timeout |
| Presigned URL expiration drift | Clock skew tolerance via SDK |
| Orphan cleanup race condition | Idempotent DELETE (no error si ya no existe) |
