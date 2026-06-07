# HU-21.2-user-invitations

**Origen:** `REQ-21-org-billing`
**Persona:** org-admin
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** owner/admin de una org
**Quiero** invitar usuarios por email con un link único de aceptación
**Para** sumar miembros sin crearles cuenta manualmente

## Criterios de aceptación

### Escenario 1: Crear invitación

```gherkin
Dado que soy admin de org "Acme"
Cuando POST /api/v1/organizations/:id/invitations con `{"email":"bob@x.com","role":"member"}`
Entonces se crea registro `invitations` con token UUIDv4
Y se envía email con link `https://app.domain.sh/accept?token=<UUID>` usando canal email-smtp
Y la invitación expira en 7 días
Y audit_log "invitation.sent"
```

### Escenario 2: Aceptar invitación

```gherkin
Dado que existe invitación válida para email "bob@x.com"
Cuando bob abre el link `?token=<UUID>` y completa el flow passwordless de HU-02.7:
  - opcionalmente proporciona su RUT en el form de aceptación
  - POST /auth/request-otp con identifier = email del invite
  - bob recibe código y POST /auth/verify-otp
Entonces se crea el user (email del invite + RUT si lo proporcionó)
Y se asigna a la org con el role del invite
Y la invitación pasa a status "accepted"
Y la respuesta de verify-otp incluye la API key del nuevo user (is_first: true)
```

### Escenario 3: Rechazar invitación

```gherkin
Dado que existe invitación pendiente
Cuando bob hace POST /api/v1/invitations/:token/decline
Entonces status = "declined"
Y no se crea user
```

### Escenario 4: Email mismatch

```gherkin
Dado que invitación es para "bob@x.com"
Cuando bob abre link y se autentica como "alice@x.com"
Entonces error "email mismatch: invitación para bob@x.com"
Y no se crea user en la org
```

### Escenario 5: Token expirado

```gherkin
Dado que invitación tiene `expires_at < now()`
Cuando se intenta aceptar
Entonces error "invitation expired"
Y status pasa a "expired" (cron diario)
```

### Escenario 6: Revocar invitación

```gherkin
Dado que admin envió invite por error
Cuando POST /api/v1/invitations/:id/revoke
Entonces status = "revoked"
Y aceptación posterior falla con "revoked"
```

## Análisis breve

- **Qué pide:** tabla invitations + endpoints + email + integración Google OAuth
- **Esfuerzo:** S
- **Riesgos:** token leak; reuso; email enumeration
