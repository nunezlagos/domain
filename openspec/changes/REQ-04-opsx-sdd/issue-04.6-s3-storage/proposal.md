# Proposal: issue-04.6-s3-storage

## Intención

Implementar integración con S3-compatible storage (AWS S3, MinIO, R2, DO Spaces) para adjuntar imágenes, diagramas y archivos a HUs, REQs y knowledge docs. Los archivos se suben vía API REST con signed URLs para descarga temporal. Soporte multi-provider vía endpoint configurable.

## Scope

**Incluye:**
- Tabla `file_attachments` con entity_type/entity_id polimórfico, s3_key, signed URL generation
- Upload directo a S3 con presigned URL (cliente sube directo a S3 sin pasar por nuestro backend)
- Download vía signed URL (1 hora de validez)
- Validación de tamaño máx 10MB y tipos permitidos (image/*, application/pdf, text/markdown)
- Cleanup diario de attachments huérfanos (entidad padre eliminada)
- Config vía env vars DOMAIN_S3_*

**Excluye:**
- CDN caching
- Thumbnails generation
- Multi-part upload para archivos grandes (>10MB se rechazan)
- Versionado de archivos

## Enfoque técnico

1. **Librería:** `github.com/aws/aws-sdk-go-v2/service/s3` con configurable endpoint (MinIO compatible)
2. **Upload flow:** Backend genera presigned PUT URL → cliente sube directo a S3 → backend confirma con HEAD
3. **Download flow:** Backend genera presigned GET URL (1h expiry) → cliente redirige o recibe URL
4. **Cleanup:** Cron diario query attachments con LEFT JOIN where padre IS NULL → delete S3 + DB
5. **Tipos permitidos:** whitelist por MIME type prefix

## Riesgos

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| Signed URL leaks | Medio | Expiración 1h; no exponer en logs |
| S3 outage | Alto | Error claro al usuario; no cache local |
| Costos storage huérfanos | Bajo | Cleanup diario automático |
| S3 credentials en env | Medio | Usar IAM role en prod; MinIO dev con env |

## Testing

- Integración con MinIO container (testcontainers)
- Upload/download flow
- File too large rejection
- File type not allowed rejection
- Cleanup de huérfanos
- Signed URL generation
