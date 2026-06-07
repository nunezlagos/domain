# Proposal: HU-21.2-user-invitations

## Intención

Sistema de invitaciones por email con token UUIDv4 de un solo uso, expiración 7 días, integración con Google OAuth login para confirmación, audit trail completo.

## Scope

**Incluye:**
- Tabla `invitations` con email, role, token, status, expires_at
- Endpoints REST send/list/get/accept/decline/revoke
- Email con template `invitation_email` (HTML + text)
- Cron diario que marca expired
- Validación email match al aceptar
- Rate-limit envío (max 50/día por org)

**No incluye:**
- Bulk import CSV (futuro)
- Self-service signup sin invite (configurable opcional)

## Enfoque técnico

1. Token: `uuid.New().String()` (no JWT, no secret)
2. Acceptance flow: redirect Google OAuth + match email
3. Email vía canal `email-smtp` con template versionado
4. Expiración via cron `0 4 * * *`

## Riesgos

- Token enumeration: 128-bit UUID v4 ya es resistente; rate-limit endpoint accept
- Email enumeration: respuesta uniforme aunque email no exista
- Reuso: status enforcement + UNIQUE constraint en token

## Testing

- Crear → email → accept → user creado y role correcto
- Email mismatch → reject
- Token expirado → reject
- Revoke → accept posterior reject
- Rate-limit 50/día respetado
