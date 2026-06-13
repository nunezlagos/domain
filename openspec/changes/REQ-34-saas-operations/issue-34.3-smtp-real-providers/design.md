# Design: issue-34.3-smtp-real-providers

## Contexto

Hoy el server usa `net/smtp` con `localhost:1025` (Mailpit en
dev). En prod eso no funciona: necesitamos proveedores REALES que
entreguen a casillas reales. La elección de proveedor depende
del operador (Resend, SES, SendGrid son los comunes).

La decisión de diseño: interface `EmailSender` con N
implementaciones, seleccionable por env var. El código que
dispara el email (OTP, invites, alerts) sigue usando la
interface — agnóstico del provider.

## Decisión arquitectónica

**Estrategia:** interface `EmailSender` con implementación
default SMTP + 3 nuevas (Resend, SES, SendGrid), selección por
env var al boot.

1. **Interface existente (refactor mínimo):**
   `internal/mail/smtp/` ya tiene `Mailer` interface. Hoy hay
   solo 1 implementación: SMTP via `net/smtp`. Lo que se
   agrega: 3 implementaciones más.

2. **Implementaciones nuevas:**
   - **`internal/mail/resend/`**:
     ```go
     type Sender struct { APIKey string; From string; HTTPClient *http.Client }
     func (s *Sender) Send(ctx, to, subject, body string) error {
       req := buildResendRequest(s.APIKey, s.From, to, subject, body)
       resp, err := s.HTTPClient.Do(req)
       if err != nil { return err }
       defer resp.Body.Close()
       if resp.StatusCode >= 400 {
         return fmt.Errorf("resend: %s", resp.Status)
       }
       return nil
     }
     ```
   - **`internal/mail/ses/`**:
     ```go
     type Sender struct { Client *ses.Client; From string }
     func (s *Sender) Send(ctx, to, subject, body string) error {
       _, err := s.Client.SendEmail(ctx, &ses.SendEmailInput{
         From: s.From, To: []string{to},
         Subject: &subject, Body: &body,
       })
       return err
     }
     ```
   - **`internal/mail/sendgrid/`**: similar a Resend pero con
     `api.sendgrid.com/v3/mail/send`.

3. **Selector en boot (cmd/domain/main.go):**
   ```go
   func newEmailSender(cfg *config.Config) (mail.Mailer, error) {
     switch cfg.SMTPProvider {
     case "", "smtp":
       return smtp.New(cfg.SMTPHost, cfg.SMTPPort, ...), nil
     case "resend":
       if cfg.ResendAPIKey == "" {
         return nil, errors.New("resend requires DOMAIN_RESEND_API_KEY")
       }
       return resend.New(cfg.ResendAPIKey, cfg.SMTPFrom), nil
     case "ses":
       if cfg.AWSRegion == "" { return nil, errors.New(...) }
       return ses.New(cfg.AWSRegion, cfg.SMTPFrom), nil
     case "sendgrid":
       if cfg.SendGridAPIKey == "" { return nil, errors.New(...) }
       return sendgrid.New(cfg.SendGridAPIKey, cfg.SMTPFrom), nil
     default:
       return nil, fmt.Errorf("unknown SMTP_PROVIDER: %s", cfg.SMTPProvider)
     }
   }
   ```

4. **Default por environment:**
   - `cfg.Env == "dev"` → default `smtp` (Mailpit).
   - `cfg.Env == "prod"` → default `resend`.
   - El user puede override con `DOMAIN_SMTP_PROVIDER`.

5. **Retry con backoff:**
   - Usar la stack de resiliencia existente (33.1) o un
     helper simple.
   - Solo retry en 429 y 5xx. 4xx (auth, validation) NO retry.
   - 3 attempts: 1s, 2s, 4s.

6. **Métricas:**
   - `metrics.EmailSent.WithLabelValues(provider, status).Inc()` —
     "resend" / "ses" / "sendgrid" / "smtp", "success" / "failed".
   - `metrics.EmailSendDuration.WithLabelValues(provider).Observe(duration)`.

7. **Testing:**
   - `httptest.Server` mockeando los endpoints de cada provider.
   - Para SES: AWS SDK tiene `LocalStack` o mocks; o usar
     `iface.ClientAPI` con un mock.

8. **docker-compose dev:** Mailpit sigue en
   `docker-compose.yml` con `ports: ["1025:1025", "8025:8025"]`.
   El server dev usa `DOMAIN_SMTP_HOST=mailpit`,
   `DOMAIN_SMTP_PORT=1025`.
   En `compose/contabo/docker-compose.yml` (issue-31.3), Mailpit
   NO se incluye.

9. **Validación de config al boot:** si el provider requiere
   config que falta, exit 1 con mensaje claro. Cero "send
   silently fails".

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | Soportar TODOS los providers (Mailgun, Postmark, etc) | Out of scope. 3 providers cubren el 95%. Se pueden agregar más. |
| B | SMTP-only (sin HTTP APIs) | Algunos providers solo tienen HTTP API (Resend). SMTP no es universal. |
| C | Un servicio de email externo (SendPulse, Customer.io) | Agrega dependencia y costo. Mejor dar opciones al operador. |
| D | Usar AWS SES SDK directo sin abstracción | Acopla a AWS. El operador puede no querer AWS. |

## Por qué interface + 3 implementaciones gana

- **Agnóstico del código de negocio:** OTP, invites, alerts
  siguen usando `mail.Mailer` — no les importa el provider.
- **Testeable:** cada implementación se mockea con httptest
  (HTTP) o iface (SES).
- **Extensible:** agregar un provider = 1 archivo nuevo + 1
  branch en el selector.
- **Backward compat:** SMTP (Mailpit) sigue funcionando para
  dev.

## Detalle de implementación

- `internal/mail/resend/sender.go` con la implementación.
- `internal/mail/ses/sender.go` con AWS SDK v2.
- `internal/mail/sendgrid/sender.go` con HTTP API.
- `internal/mail/selector.go` con `NewFromConfig(cfg) (Mailer, error)`.
- Agregar a `config.Config`:
  - `SMTPProvider string` (env: `DOMAIN_SMTP_PROVIDER`).
  - `ResendAPIKey string`.
  - `SendGridAPIKey string`.
  - `AWSRegion string` (reusar la de issue-31.3 si ya existe).
- Wire en `runServer` y `runInstall` (para validaciones de
  email).
- Métricas: en cada `Send` exitoso/fallido.

## Riesgos

- **R1:** API keys en env vars pueden leakear a logs. **Mitigación:**
  los `Sender` solo usan la key internamente; nunca la loggean.
  El log de "email sent" solo tiene provider + to + subject.
- **R2:** Resend free tier tiene límites (100/día). **Aceptable:**
  documentar. Operadores grandes usan SES.
- **R3:** SES sandbox requiere verificar emails. **Documentar:**
  en sandbox, solo emails a direcciones verificadas se
  entregan. Operador debe mover a producción.
