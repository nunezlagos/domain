# Proposal: issue-23.3-gdpr-export

## Intención

Endpoint GDPR-compliant para que cualquier user descargue export ZIP con todos sus datos en formato JSON portable, adjuntos incluidos, signed URL temporal por 24h, notificación email cuando esté listo.

## Scope

**Incluye:**
- Tabla `export_jobs` con status, progress, expires_at, signed_url
- Endpoint POST /me/export + GET /me/exports/:job_id
- Worker async que serializa, comprime, sube a S3 dedicado, signed URL
- Notificación email (canal email-smtp)
- README.md auto-generado en el ZIP
- Rate-limit 1 export/24h por user

**No incluye:**
- Right to be forgotten (otra HU futura: hard-delete cuenta)
- Export bulk de admin (separado)

## Enfoque técnico

1. Streaming JSON write (no acumular todo en memoria)
2. Streaming ZIP (archive/zip stdlib)
3. Upload S3 multipart si >100MB
4. Signed URL con expiración 24h vía S3 presign
5. Email con link de descarga + checksum SHA256

## Riesgos

- PII de terceros: cuidadoso scoping de queries; tests adversariales
- Tamaño: pre-cálculo y warning si >500MB
- Crash mid-job: status partial, retry desde último checkpoint opcional

## Testing

- Export user con datos → ZIP válido con todas las claves esperadas
- Test adversarial: user A exporta → datos de user B NO presentes
- Rate-limit 2do export en 24h
- ZIP descomprimible y parseable
