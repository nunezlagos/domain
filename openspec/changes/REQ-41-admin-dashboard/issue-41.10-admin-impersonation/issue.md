# issue-41.10-admin-impersonation

**Origen:** `REQ-41-admin-dashboard`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** super_admin de la plataforma
**Quiero** entrar al panel como si fuera un user específico de una org (impersonation), con un banner visible y doble audit, para dar soporte y debugging
**Para** reproducir el problema de un user sin pedirle credenciales ni generar cuentas temporales

## Criterios de aceptación

```gherkin
Feature: Impersonation (Super Admin Debug)

  Background:
    Given el usuario está autenticado con rol super_admin
    And el super_admin tiene un motivo válido (soporte, debugging)

  Scenario: Modal de confirmación antes de impersonar
    Given estoy en /admin/cross-org y veo la tabla de orgs
    When hago clic en "Entrar como admin" de Org A
    Then se abre ModalComponent de confirmación con:
      | campo | contenido |
      | Header | "Entrar como owner de [Org A]" |
      | Warning Alert | "Estás a punto de impersonar al owner de esta org. TODAS las acciones que hagas se registrarán en el audit log con tu identidad real. Solo para soporte/debugging." |
      | Checkbox | "Entiendo que esto se registra para compliance" (required) |
      | Botón "Entrar" | disabled hasta check |
      | Botón "Cancelar" | cierra modal |
    Y al confirmar, llama POST /api/v1/admin/impersonate con {target_user_id, org_id, reason}

  Scenario: Backend crea sesión de impersonation
    Given la confirmación es válida
    When POST /api/v1/admin/impersonate se ejecuta
    Then el backend:
      | paso | descripción |
      | 1 | Valida que caller sea super_admin |
      | 2 | Valida que target_user existe y pertenece a org_id |
      | 3 | Genera un session_token con `impersonated_by=<super_admin_id>` claim |
      | 4 | Persiste un registro en `impersonation_sessions` con (super_admin_id, target_user_id, org_id, started_at, reason, expires_at = +2h) |
      | 5 | Emite audit log: action=`impersonation.started`, actor=super_admin_id, target=target_user_id, org=org_id, metadata={reason} |
      | 6 | Devuelve {session_token, expires_at, target_user, target_org, impersonation_id} |

  Scenario: Banner sticky de impersonation
    When la sesión de impersonation está activa
    Then veo un banner sticky arriba de toda la UI:
      | campo | contenido |
      | Background | amarillo (variant warning) |
      | Icono | cil-warning |
      | Texto | "Estás impersonando a [user.name] en [org.name]. Inicio: [timestamp]. Motivo: [reason]. Click acá para salir." |
      | Botón "Salir" | rojo, llama POST /api/v1/admin/impersonate/stop |
    Y el banner NO es dismissable (es permanente mientras dure la impersonation)
    Y el sidebar muestra un badge "IMPERSONATING" en el item de org actual

  Scenario: Acciones durante impersonation
    Given estoy impersonando
    When hago cualquier acción (crear member, editar settings, etc.)
    Then la acción se registra en audit log con:
      | campo | valor |
      | actor_id | super_admin_id (REAL, no el impersonated) |
      | impersonated_user_id | target_user_id |
      | org_id | target_org_id |
      | action | la acción normal |
    Y el request header `Authorization: Bearer <token>` tiene el token de impersonation
    Y el backend loggea ambos IDs

  Scenario: Expiración automática
    Given la sesión de impersonation tiene expires_at = +2h
    When pasan 2h sin renovar
    Then el token expira
    Y el backend retorna 401 a cualquier request
    Y el frontend redirige a /login con mensaje "Sesión de impersonation expirada"

  Scenario: Salir de impersonation
    When hago clic en "Salir" del banner
    Then se abre ModalComponent de confirmación "¿Salir de impersonation? Volverás a tu sesión de super_admin."
    Y al confirmar, llama POST /api/v1/admin/impersonate/stop
    Y el backend:
      | paso | descripción |
      | 1 | Marca `impersonation_sessions.ended_at = now()` |
      | 2 | Invalida el session_token de impersonation |
      | 3 | Emite audit log: action=`impersonation.stopped`, actor=super_admin_id, impersonation_id |
      | 4 | Devuelve {restored_session_token} (token original del super_admin) |
    Y el frontend actualiza el auth.service con el token restaurado
    Y el header vuelve a "Todas las orgs" o al org del super_admin
    Y el banner desaparece
    Y un toast "Saliste de impersonation" se muestra

  Scenario: Ver historial de impersonations
    Given soy super_admin
    When navego a /admin/audit?actor_type=super_admin&action=impersonation.*
    Then veo la lista de impersonations pasadas con:
      | columna | valor |
      | Inicio | started_at |
      | Fin | ended_at (o "activa" si sigue) |
      | Super admin | nombre del super_admin que impersonó |
      | User impersonado | target_user.email |
      | Org | target_org.name |
      | Motivo | reason (textarea del modal) |
      | Duración | now - started_at (si ended_at) |
    Y cada fila tiene link al detalle del audit log

  Scenario: Intento de impersonar sin ser super_admin
    Given soy admin (no super_admin)
    When intento llamar POST /api/v1/admin/impersonate
    Then el backend retorna 403 Forbidden
    Y no se crea ninguna sesión

  Scenario: Impersonar a otro super_admin (no permitido)
    Given soy super_admin A
    When intento impersonar a super_admin B
    Then el backend retorna 400 Bad Request "No se puede impersonar a otro super_admin"
    Y se registra en audit como intento bloqueado

  Scenario: Rate limit de impersonation
    Given soy super_admin
    When intento iniciar más de 5 impersonations en 1 hora
    Then el backend retorna 429 Too Many Requests
    Y un mensaje "Demasiadas impersonations. Esperá 1h."

  Scenario: Banner de impersonation en TODAS las vistas
    When navego a /admin/dashboard, /admin/members, /admin/audit, etc.
    Then el banner sigue visible arriba
    Y NO tapa el navbar (es parte del layout, no un overlay)
```

## Componentes del template CoreUI a reusar

| Componente | Path | Uso |
|---|---|---|
| `AlertComponent` (variant warning, sticky) | `views/notifications/alerts/` | Banner de impersonation (customizado para ser sticky arriba) |
| `ModalComponent` + set completo | `views/notifications/modal/` | Modales de iniciar impersonation, confirmar salida, ver detalle |
| `FormControlDirective` + `FormLabelDirective` | `views/forms/form-control/` | Textarea de reason |
| `FormCheckComponent` | `views/forms/checks-radios/` | Checkbox de "entiendo" en modal de iniciar |
| `ButtonDirective` | `views/buttons/` | Botón "Salir" del banner |
| `TableDirective` | `views/base/tables/` | Tabla de historial de impersonations |
| `BadgeComponent` | `views/notifications/badges/` | Badge "IMPERSONATING" en el sidebar |
| `SpinnerComponent` | `views/base/spinners/` | Loading durante start/stop |
| `ToasterComponent` | `views/notifications/toasts/` | Feedback de "Saliste de impersonation" |
| `IconDirective` (cil-*) | `@coreui/angular` | Icono `cil-warning` del banner, `cil-exit-to-app` del botón salir |

**Custom component nuevo**: `ImpersonationBannerComponent` que se monta en el `DefaultLayoutComponent` y se muestra cuando `AuthService.isImpersonating()` es true. Hace polling cada 30s a `GET /api/v1/admin/impersonate/active` para refrescar el estado.

## Endpoints del backend

| Endpoint | Acción | Estado |
|---|---|---|
| `POST /api/v1/admin/impersonate` | Inicia impersonation | **A CREAR** en esta HU |
| `POST /api/v1/admin/impersonate/stop` | Sale de impersonation | **A CREAR** en esta HU |
| `GET /api/v1/admin/impersonate/active` | Estado actual de impersonation | **A CREAR** en esta HU |
| `GET /api/v1/audit-logs?action=impersonation.*` | Historial de impersonations | ya existe (con filtros) |

**Endpoints nuevos a crear**:

### `POST /api/v1/admin/impersonate`
```json
// Request
{
  "target_user_id": "uuid",
  "org_id": "uuid",
  "reason": "Bug report #1234: user no ve sus agentes"
}

// Response 200
{
  "impersonation_id": "uuid",
  "session_token": "...",
  "expires_at": "2026-06-16T18:00:00Z",
  "target_user": { "id": "...", "email": "...", "name": "..." },
  "target_org": { "id": "...", "name": "...", "slug": "..." }
}

// Errors
// 403: caller no es super_admin
// 400: target es super_admin
// 429: rate limit (>5/hora)
// 404: target_user o org no existe
```

### `POST /api/v1/admin/impersonate/stop`
```json
// Response 200
{
  "restored_session_token": "..."
}
```

### `GET /api/v1/admin/impersonate/active`
```json
// Response 200 (no impersonation activa)
{ "active": false }

// Response 200 (impersonation activa)
{
  "active": true,
  "impersonation_id": "uuid",
  "super_admin": { "id": "...", "email": "..." },
  "target_user": { "id": "...", "email": "...", "name": "..." },
  "target_org": { "id": "...", "name": "..." },
  "started_at": "...",
  "expires_at": "...",
  "reason": "..."
}
```

**Schema nuevo** (migración):
```sql
CREATE TABLE impersonation_sessions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  super_admin_id UUID NOT NULL REFERENCES users(id),
  target_user_id UUID NOT NULL REFERENCES users(id),
  org_id UUID NOT NULL REFERENCES organizations(id),
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  ended_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ NOT NULL,
  reason TEXT NOT NULL,
  created_session_token_hash TEXT NOT NULL  -- para poder revocar
);
CREATE INDEX idx_impersonation_sessions_super_admin ON impersonation_sessions(super_admin_id, started_at DESC);
CREATE INDEX idx_impersonation_sessions_target ON impersonation_sessions(target_user_id, started_at DESC);
CREATE INDEX idx_impersonation_sessions_active ON impersonation_sessions(expires_at) WHERE ended_at IS NULL;
```

## Análisis breve

- **Qué pide realmente:** El feature más sensible del admin. Permite a un super_admin entrar como cualquier user, con trazabilidad completa. Es crítico para soporte y debugging en producción.
- **Módulos a tocar:** Backend: `internal/api/handler/admin/impersonate_handler.go`, migración nueva `impersonation_sessions`, middleware que adjunta `impersonated_by` al context para audit. Frontend: nuevo `ImpersonationBannerComponent`, modal de start/stop, vista de historial (en /admin/audit con filtros).
- **Riesgos / dependencias:** Máxima seguridad. Validar TODOS los casos (no super_admin, target es super_admin, rate limit, expiración). El token de impersonation NO debe poder usarse para crear otros super_admins. El audit DEBE registrar SIEMPRE la identidad real del super_admin, no la del user impersonado. El `impersonation_sessions.ended_at` debe quedar persistido aunque la sesión expire por TTL.
- **Esfuerzo tentativo:** L

## Verificación previa

- [ ] Confirmar que el middleware actual distingue super_admin de admin
- [ ] Verificar cómo se genera el session_token hoy (estructura del JWT o token opaco)
- [ ] Decidir: ¿el token de impersonation es un JWT con claim extra o un token opaco nuevo en `api_keys`?
- [ ] Confirmar el rate limit: ¿5/hora está bien o ajustar?
- [ ] Verificar que el TTL de 2h es razonable (¿1h? ¿4h?)
- [ ] Decidir si el super_admin puede tener varias impersonations simultáneas (recomendación: NO, una a la vez)
- [ ] Confirmar que el audit log ya soporta los campos `impersonated_user_id` y `actor_id` separados (probable migración)
- [ ] Testear el caso edge: super_admin A impersona a user X, user X es promovido a super_admin mid-session, ¿qué pasa?

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
