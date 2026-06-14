# Design: issue-20.2-email-smtp

## Decisión arquitectónica

**Lib:** `github.com/wneessen/go-mail` v0.5+
**Razón:** API limpia, multipart correcto, DKIM built-in, fewer footguns que stdlib

## Alternativas descartadas

- **stdlib net/smtp:** muy bajo nivel, no maneja multipart bien
- **gomail/v2:** archivado/sin mantener
- **Sendgrid API:** lock-in vendor; futuro canal separado si se necesita

## Componentes

```
internal/notifications/channels/email/
  channel.go     # implements notifications.Channel
  config.go      # SMTPConfig struct
  dkim.go        # firma opcional
  validator.go   # email validation
```

## Variables de entorno

| var | default | descripción |
|-----|---------|-------------|
| DOMAIN_SMTP_HOST | localhost | |
| DOMAIN_SMTP_PORT | 1025 | 587 prod, 1025 dev |
| DOMAIN_SMTP_AUTH | none | none\|plain\|login\|cram-md5 |
| DOMAIN_SMTP_USER | | |
| DOMAIN_SMTP_PASSWORD | | |
| DOMAIN_SMTP_TLS | false (dev) | STARTTLS |
| DOMAIN_SMTP_FROM | no-reply@domain.local | |
| DOMAIN_SMTP_DKIM_PRIVATE_KEY_PATH | | DKIM opcional |
| DOMAIN_SMTP_DKIM_SELECTOR | domain | |
| DOMAIN_SMTP_DKIM_DOMAIN | | |

## TDD plan

1. Unit con mock smtp server → mensaje correcto
2. Integration Mailpit en dev → aparece en UI
3. Validación email inválido fail-fast
4. DKIM signing presente cuando configurado
