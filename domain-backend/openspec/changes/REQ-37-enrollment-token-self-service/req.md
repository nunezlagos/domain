# REQ-37 — Enrollment Token Self-Service

> **Origen**: sesión 2026-06-13. Decisión: el deploy cloud (rama `services`)
> no tiene SMTP/dominio. El flujo OTP+email queda preservado pero apagado.
> Mientras tanto, necesitamos que los users SE AUTO-ENROLEN con un token
> compartido — sin que un admin tenga que crear cada user a mano.

## Contexto

Estado actual:
- `bootstrap` (issue-01.9) crea el primer user del sistema.
- `invitations` (issue-21.2) requiere SMTP — apagado en cloud.
- `AddMemberWithAPIKey` (issue-36.1) requiere admin existente que crea
  user-por-user a mano. Funciona pero no escala.

REQ-37 introduce un **enrollment token compartido por org**: cualquiera
con el token se enrola solo, obtiene su propia API key personal y queda
adentro. Reemplaza temporalmente al flujo email/OTP hasta que esté la infra
de 2FA.

## Modelo conceptual

```
┌─ admin (Mauricio) ──────────────────────────────────────────┐
│ POST /organizations/{id}/enrollment-token/rotate            │
│ → recibe token plaintext UNA vez: "et_a1b2c3..."           │
└──────────────────────────────────────────────────────────────┘
              │ comparte por Slack / mensaje
              ▼
┌─ user nuevo (Alice) ─────────────────────────────────────────┐
│ POST /auth/enroll                                            │
│   header: X-Enrollment-Token: et_a1b2c3...                  │
│   body:   {email, name}                                      │
│ ← recibe SU api_key personal UNA vez: "domk_live_..."       │
└──────────────────────────────────────────────────────────────┘
```

Si el token se filtra: el admin rota, el viejo queda inválido.

## Issues

| Issue | Slug | Esfuerzo | Descripción |
|-------|------|----------|-------------|
| 37.1 | `self-enroll-shared-token` | M | Tabla `org_enrollment_tokens` + endpoint público `POST /auth/enroll` + endpoints admin `rotate/get/delete`. Bootstrap (issue-01.9) extendido para emitir el primer token automáticamente. |

## Decisiones de diseño

- **Multi-use con rotación manual**: el mismo token sirve para N enrollments
  hasta que el admin lo rote. Filtración → rotación manual. Más cómodo que
  single-use.
- **Scoped por org**: cada org tiene su token (no global). Multi-tenant clean.
- **Role configurable**: al rotar, el admin elige el role que tendrán los
  enrollees (`member` por default). Útil para crear cohortes de role distinto.
- **Sin expiración automática**: rotación 100% manual = control explícito.
  En el futuro se puede agregar TTL opcional.
- **Storage en DB**: tabla `org_enrollment_tokens`, NO `.env`. Rotable vía
  HTTP sin restart del server.
- **Bootstrap emite el primer token**: al crear org+owner en first-run, también
  genera token con role_on_enroll="member" y lo incluye en el response del
  bootstrap. El owner ya tiene todo listo para invitar a su equipo.

## Coexistencia con flujos existentes

| Flujo | Estado | Cuándo usarlo |
|-------|--------|---------------|
| Bootstrap (01.9) | Activo | First-run del install. Crea org+owner+token. |
| Invitations + OTP (21.2) | Apagado en cloud | Cuando vuelva SMTP + dominio + 2FA. |
| AddMemberWithAPIKey (36.1) | Activo | Admin crea user específico desde curl/CLI. |
| Self-enrollment (37.1) | **Nuevo** | User se enrola solo con token compartido. |

Ninguno deprecia al otro. El admin elige caso por caso.

## Cuando llegue el SMTP/2FA (REQ futuro)

- El `enrollment_token` puede ser revocado globalmente (`DELETE` en todos).
- Los users existentes siguen con sus api keys.
- El flow vuelve a ser invitations + OTP + 2FA.

## Prioridad: **alta**

Bloqueante para uso real del deploy cloud por más de 1 user.
