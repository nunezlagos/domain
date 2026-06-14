# Tasks: issue-34.3-smtp-real-providers

## Backend

- [ ] **T1]: Crear `internal/mail/resend/sender.go`:
  - `type Sender struct { APIKey, From string; HTTPClient *http.Client; Logger *slog.Logger }`.
  - `Send(ctx, to, subject, body string) error`.
  - Endpoint: `POST https://api.resend.com/emails` con Bearer.
  - Body: `{from, to: [to], subject, text: body}`.
  - 200 → success, 4xx/5xx → error con detalle.

- [ ] **T2]: Crear `internal/mail/ses/sender.go`:
  - Dep: `github.com/aws/aws-sdk-go-v2/service/sesv2`.
  - `type Sender struct { Client *sesv2.Client; From string; Logger *slog.Logger }`.
  - `Send(ctx, to, subject, body string) error`.
  - Llama `SendEmail` con `Content: {Simple: {Subject, Body: {Text: {Data: body}}}}`.

- [ ] **T3]: Crear `internal/mail/sendgrid/sender.go`:
  - `type Sender struct { APIKey, From string; HTTPClient *http.Client }`.
  - Endpoint: `POST https://api.sendgrid.com/v3/mail/send`.
  - Personalizations[0].to[0].email = to.
  - 202 → success (SendGrid usa 202 Accepted).

- [ ] **T4]: Crear `internal/mail/selector.go`:
  - `type Config struct { Provider, Host, Port, User, Password, From, ResendAPIKey, SendGridAPIKey, AWSRegion string }`.
  - `NewFromConfig(cfg Config, env string, logger *slog.Logger) (Mailer, error)`.
  - Switch sobre `cfg.Provider` (default por env).

- [ ] **T5]: Refactor de `internal/mail/smtp/` (ya existe) para
  encajar en la interface `Mailer` (probablemente ya lo hace).
  Verificar.

- [ ] **T6]: Agregar a `config.Config`:
  - `SMTPProvider string` (env: `DOMAIN_SMTP_PROVIDER`).
  - `ResendAPIKey string` (env: `DOMAIN_RESEND_API_KEY`).
  - `SendGridAPIKey string` (env: `DOMAIN_SENDGRID_API_KEY`).
  - `AWSRegion string` (env: `AWS_REGION`).
  - Reusar `SMTPHost`, `SMTPPort`, `SMTPUser`, `SMTPPassword`,
    `SMTPFrom` (ya existen).

- [ ] **T7]: Wire en `cmd/domain/main.go` `runServer`:
  ```go
  mailer, err := mail.NewFromConfig(mail.Config{
    Provider: cfg.SMTPProvider, /* ... */
  }, cfg.Env, logger)
  if err != nil { logger.Error("mailer init failed", slog.Any("err", err)); os.Exit(1) }
  ```
  Reemplazar el `smtpmail.New(...)` actual.

- [ ] **T8`: Métricas:
  - `metrics.EmailSent.WithLabelValues(provider, status).Inc()`.
  - `metrics.EmailSendDuration.WithLabelValues(provider).Observe(d)`.

- [ ] **T9`: Retry helper en `internal/mail/retry.go`:
  - 3 attempts, backoff 1s/2s/4s.
  - Solo retry en 429 y 5xx. 4xx → fail immediate.

- [ ] **T10`: Actualizar `docker-compose.yml` dev con
  documentación: "Mailpit incluido solo para dev. En prod, usar
  `DOMAIN_SMTP_PROVIDER=resend` (o ses/sendgrid)".
  Actualizar `compose/contabo/docker-compose.yml` (issue-31.3) para
  NO incluir Mailpit.

- [ ] **T11`: Actualizar `.env.example` con las nuevas env vars
  (commented out con explicación).

## Tests

- [ ] `TestResend_SendOK**` — httptest server que mockea
  api.resend.com y retorna 200 → Send success.
- [ ] `TestResend_SendFails_4xx**` — mock retorna 401 → Send
  retorna error, NO retry (4xx).
- [ ] `TestResend_RetriesOn5xx**` — mock retorna 503, 503, 200 →
  Send success después de 2 retries.
- [ ] `TestResend_NoLeakAPIKey**` — mock captura el request, el
  log NO contiene el API key (verificar con grep).
- [ ] `TestSES_SendOK**` — mock SES client con
  `SendEmail` que retorna success → Send OK.
- [ ] `TestSendGrid_SendOK**` — mock con 202 → success.
- [ ] `TestSelector_DefaultsToSMTPInDev**` — env=dev, no
  Provider → returns smtp.Sender.
- [ ] `TestSelector_DefaultsToResendInProd**` — env=prod, no
  Provider → returns resend.Sender.
- [ ] `TestSelector_RequiresResendKey**` — Provider=resend, sin
  APIKey → returns error.
- [ ] `TestSelector_RequiresSESRegion**` — Provider=ses, sin
  AWSRegion → returns error.
- [ ] `TestE2E_OTPUsesResend**` — flow OTP end-to-end con
  provider=resend mockeado → el mock recibe el POST a
  api.resend.com con Bearer.
- [ ] `T-sabotaje`: Comentar la rama `case "resend"` en el
  selector (sabotaje: siempre cae a smtp) → test e2e OTPUsesResend
  DEBE FALLAR (la llamada va a localhost:1025, no a api.resend.com)
  → restaurar rama → test verde. Documentar en commit body.
