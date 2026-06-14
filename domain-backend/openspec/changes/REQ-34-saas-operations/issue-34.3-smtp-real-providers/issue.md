# issue-34.3-smtp-real-providers

**Origen:** `REQ-34-saas-operations`
**Prioridad tentativa:** media
**Tipo:** feature (operational)

## Historia de usuario

**Como** operador del VPS en producción
**Quiero** que domain use un proveedor SMTP real (Resend, AWS SES, o SendGrid) en vez de Mailpit
**Para** que los OTP emails lleguen a casillas reales de clientes (no se queden en un mailbox local de dev)

## Criterios de aceptación

### Escenario 1: Resend como provider

```gherkin
Dado que `DOMAIN_SMTP_PROVIDER=resend` + `DOMAIN_RESEND_API_KEY=re_xxx` + `DOMAIN_SMTP_FROM=noreply@tudominio.com`
Cuando el server envía un OTP email
Entonces el email se envía via Resend API (`POST https://api.resend.com/emails` con Bearer auth)
Y Resend lo entrega al destinatario real
Y el response de Resend se loggea (status 200 + id del email)
```

### Escenario 2: AWS SES como provider

```gherkin
Dado que `DOMAIN_SMTP_PROVIDER=ses` + `AWS_REGION=us-east-1` + credenciales en el env standard AWS_* + `DOMAIN_SES_FROM=noreply@tudominio.com`
Cuando el server envía un OTP email
Entonces el email se envía via AWS SES SDK
Y SES lo entrega (o lo pone en sandbox si el recipient no está verified)
Y el response se loggea
```

### Escenario 3: SendGrid como provider

```gherkin
Dado que `DOMAIN_SMTP_PROVIDER=sendgrid` + `DOMAIN_SENDGRID_API_KEY=SG.xxx` + `DOMAIN_SMTP_FROM=noreply@tudominio.com`
Cuando el server envía un OTP email
Entonces el email se envía via SendGrid API
Y SendGrid lo entrega
Y el response se loggea
```

### Escenario 4: Mailpit SOLO en dev

```gherkin
Dado que estoy en dev local con `docker-compose up`
Cuando arranco el server
Entonces Mailpit está corriendo (docker compose) y recibe los emails
Y el server usa SMTP localhost:1025 (config dev)
Y los emails se ven en http://localhost:8025 (Mailpit UI)
Y NO se hace llamada a Resend/SES/SendGrid
```

### Escenario 5: Default en prod = Resend

```gherkin
Dado que `DOMAIN_SMTP_PROVIDER` NO está seteada
Y el binario se compila con `-ldflags "-X main.Env=prod"`
Cuando el server arranca
Entonces el provider default es Resend
Y loggea: "SMTP provider: resend (default for prod)"
Y si falta `DOMAIN_RESEND_API_KEY` → error claro y exit 1
```

### Escenario 6: Validación al boot

```gherkin
Dado que `DOMAIN_SMTP_PROVIDER=resend` pero falta `DOMAIN_RESEND_API_KEY`
Cuando el server arranca
Entonces exit 1 con mensaje: "SMTP provider 'resend' requires DOMAIN_RESEND_API_KEY"
Y el server NO arranca (no quiere fallar emails silenciosamente)
```

### Escenario 7: Provider rate limit (429) → retry con backoff

```gherkin
Dado que Resend retorna 429 (rate limit)
Cuando el server intenta enviar
Entonces reintenta 3 veces con backoff (1s, 2s, 4s)
Y si los 3 fallan, retorna error al caller (OTP flow falla con
mensaje claro al user: "could not send email, try again")
```

### Escenario 8: Sabotaje — provider smtp real pero config apunta a Mailpit

```gherkin
Dado que `DOMAIN_SMTP_PROVIDER=resend` está seteado (prod mode)
Y el código tiene un bug (sabotaje) que SIEMPRE usa SMTP localhost:1025
Y Resend no se llama
Cuando se envía un OTP
Entonces el email va a Mailpit (no llega al user real)
Y el test e2e que assserta "con SMTP_PROVIDER=resend, la llamada
va a api.resend.com" DEBE FALLAR
Cuando restauro la lógica de selección por provider
Entonces el test verde
```

### Escenario 9: Cost tracking del provider

```gherkin
Dado que envío 100 emails via Resend
Cuando termina
Entonces `cost_logs` o un counter `metrics.EmailSent.WithLabelValues(provider)`
registra: 100 emails, provider=resend
Y el admin puede ver en /usage (33.4) "emails sent: 100"
```

## Notas

- Resend es el default en prod porque: (a) free tier generoso
  (100/día, 3K/mes), (b) API simple HTTP, (c) docs claras.
- AWS SES es más barato a escala (~$0.10/1000 emails) pero
  requiere sandbox setup.
- SendGrid es la opción enterprise.
- Mailpit queda SOLO en `docker-compose.yml` para dev. En
  prod compose (issue-31.3), no se levanta.
