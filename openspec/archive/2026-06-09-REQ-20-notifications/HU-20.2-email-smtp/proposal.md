# Proposal: HU-20.2-email-smtp

## Intención

Canal `email-smtp` que implementa `NotificationChannel` con soporte SMTP+TLS, multipart HTML/text, DKIM opcional, validación de email y smoke testing contra Mailpit en dev.

## Scope

**Incluye:**
- Lib `github.com/wneessen/go-mail` v0.5+ (más segura que stdlib net/smtp)
- Configuración via env DOMAIN_SMTP_*
- Templates con subject/body html/body text
- DKIM signing opcional con clave RSA
- Validación email con `github.com/wneessen/go-mail/parser`
- Smoke test wired contra Mailpit en compose dev

**No incluye:**
- Inbox/IMAP/bounce handling
- Sendgrid/SES API directos (futuro como canales separados)

## Enfoque técnico

1. Implementación stateless: cada Send abre conexión fresca o usa pool de N conexiones
2. STARTTLS forzado si auth != none
3. Templates renderizados antes de Send (no en el canal)

## Riesgos

- Conexión SMTP slow → timeout 30s y retry vía worker
- Deliverability: documentar SPF/DKIM/DMARC config externa
- Lib third-party: pinear versión + audit

## Testing

- Unit con server SMTP mock
- Integration contra Mailpit en dev compose
- Validación email inválido → fail-fast
