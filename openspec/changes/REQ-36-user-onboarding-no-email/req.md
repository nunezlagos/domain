# REQ-36 — User Onboarding sin SMTP

> **Origen**: sesión 2026-06-13. Decisión arquitectónica: el deploy cloud
> (rama `services`) no incluye SMTP. El flujo OTP/email queda inservible.
> Necesitamos un onboarding alternativo donde el admin de la org crea users
> directamente con una API key auto-generada que entrega por el canal que
> tenga (Slack, mensaje, en persona).

## Contexto

Hoy el onboarding requiere SMTP en 2 lugares:

```
1. Bootstrap (issue-01.9 first-run): el primer user de la DB se crea sin
   email — bootstrap genera API key directo. OK.

2. Invitations (issue-21.2): el admin invita por email; el user recibe link
   con token; acepta vía OTP enviado por SMTP. FALLA sin SMTP.
```

Sin SMTP en cloud, los users adicionales NO se pueden crear. El admin queda
trabado en una org de 1 user.

REQ-36 introduce un **flow paralelo** al de invitations: el admin crea el
user directamente, el server responde con la API key plaintext UNA vez, el
admin se la pasa al user por el canal que tenga.

## Issues

| Issue | Slug | Esfuerzo | Descripción |
|-------|------|----------|-------------|
| 36.1 | `create-member-with-api-key` | M | `POST /api/v1/organizations/{id}/members` (admin-only). Body `{email, name, role}`. Atomicidad: crea user + api_key en una sola tx. Response: `{user, api_key_plaintext}` — el plaintext aparece UNA sola vez. |

## Compatibilidad con invitations

REQ-36 **no reemplaza** el flow de invitations. Coexisten:
- Si la org tiene SMTP configurado y quiere validar el email del invitado →
  usar `POST /invitations` (flow viejo)
- Si no hay SMTP o el admin prefiere onboarding directo → usar
  `POST /organizations/{id}/members` (flow nuevo)

El admin decide caso por caso.

## Seguridad

- Solo admins/owners de la org pueden invocar este endpoint
- El plaintext de la API key aparece UNA sola vez en el response (mismo
  patrón que bootstrap y `POST /api-keys` existente)
- Audit log obligatorio: `member.created_with_key` con actor, target email,
  role, key_prefix
- Sin email verification: si el admin escribe mal el email, el user queda
  asociado a un email inválido pero la key sigue siendo válida. Tradeoff
  aceptado por la decisión de no SMTP.

## Prioridad: **alta**

Bloqueante para usar domain en el VPS multi-user antes de tener SMTP real.
