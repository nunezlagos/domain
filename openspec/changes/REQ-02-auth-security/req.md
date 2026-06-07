# REQ-02-auth-security: Autenticación por API keys, RBAC (admin/developer/viewer), secrets encryption, audit log inmutable, rate limiting, PII redaction.

**Estado:** activo
**Creado:** 2026-06-07
**Fase:** F1, F3

## Descripción

Autenticación por API keys, RBAC (admin/developer/viewer), secrets encryption, audit log inmutable, rate limiting, PII redaction.

## Criterios de éxito

- API keys seguras con bcrypt, prefijo, CRUD, rotación y revocación
- RBAC funcional con roles admin/developer/viewer y permisos por entidad
- Secrets encriptados con AES-256-GCM y soporte de rotación
- Audit log inmutable con consultas y retention policy
- Rate limiting por API key + PII redaction automática
- Activity log cronológico de todas las operaciones, filtrable por proyecto/usuario/entidad
- Login passwordless por OTP de 6 dígitos al email. Identifier: RUT chileno (validado módulo 11) o email. Solo invitación admin (sin self-signup). Una API key activa por usuario con opción "reveal" o "regenerate" al validar OTP
- Custom roles per-organización con matriz fine-grained de permisos por (resource, action) y scoping opcional por IDs (Enterprise)

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| HU-02.1-api-key-auth | proposed | API key generation, bcrypt hashing, CRUD, rotación, middleware |
| HU-02.2-rbac | proposed | Roles admin/developer/viewer, permisos por entidad, org scoping |
| HU-02.3-secrets-encryption | proposed | AES-256-GCM encrypt/decrypt, key rotation |
| HU-02.4-audit-log | proposed | Immutable audit trail, queries, retention 90d |
| HU-02.5-rate-limit-pii | proposed | Token bucket rate limit + PII redaction |
| HU-02.6-activity-log | proposed | Activity log general: quién, qué, cuándo, filtrable por proyecto/usuario/entidad |
| HU-02.7-passwordless-otp-auth | proposed | Login passwordless OTP por email + RUT/email identifier, devuelve API key (reveal o regenerate) |
| HU-02.8-custom-roles-permissions | proposed | Custom roles per-org con matriz fine-grained de permisos resource×action, scope opcional por IDs |
