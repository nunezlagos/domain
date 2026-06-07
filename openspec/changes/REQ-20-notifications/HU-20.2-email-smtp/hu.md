# HU-20.2-email-smtp

**Origen:** `REQ-20-notifications`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** plataforma
**Quiero** un canal email-SMTP que cumpla la interfaz `NotificationChannel`
**Para** enviar invitaciones, alertas y reportes a usuarios por mail

## Criterios de aceptación

### Escenario 1: Envío SMTP exitoso

```gherkin
Dado que `DOMAIN_SMTP_HOST`, `_PORT`, `_USER`, `_PASSWORD`, `_FROM` están configurados
Y existe template `invitation_email` con subject + body HTML + text
Cuando se invoca `email.Send(ctx, msg)` con recipient válido
Entonces se envía al servidor SMTP con TLS (STARTTLS)
Y se incluyen headers Message-ID, Date, From, To, Subject
Y multipart/alternative con html + text
Y `notification_deliveries.status = "sent"`
```

### Escenario 2: Validación de email

```gherkin
Dado que `recipient = "no-arroba"`
Cuando se invoca Send
Entonces falla inmediatamente con error "invalid email"
Y no se reintenta (4xx-like)
```

### Escenario 3: Soporte SMTP plain (dev) y autenticado (prod)

```gherkin
Dado que `DOMAIN_SMTP_AUTH=none` (dev con Mailpit)
Cuando se envía
Entonces no se intenta auth
Y conecta sin TLS si `DOMAIN_SMTP_TLS=false`

Dado que `DOMAIN_SMTP_AUTH=plain` (prod)
Cuando se envía
Entonces auth PLAIN + STARTTLS obligatorio
```

### Escenario 4: DKIM signing opcional

```gherkin
Dado que `DOMAIN_SMTP_DKIM_PRIVATE_KEY_PATH` está configurado
Cuando se envía
Entonces el mensaje se firma DKIM antes de enviar
Y el header `DKIM-Signature` está presente
```

### Escenario 5: Smoke contra Mailpit en dev

```gherkin
Dado que el stack dev está corriendo (HU-01.6)
Cuando se envía un email a "test@example.com"
Entonces aparece en la UI de Mailpit en http://localhost:8025
```

## Análisis breve

- **Qué pide:** stdlib `net/smtp` o `wneessen/go-mail` con TLS + DKIM opcional
- **Esfuerzo:** S
- **Riesgos:** deliverability (SPF/DKIM/DMARC) en prod; gestión de bounces fuera de scope
