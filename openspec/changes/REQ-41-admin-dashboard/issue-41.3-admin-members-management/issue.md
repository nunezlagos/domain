# issue-41.3-admin-members-management

**Origen:** `REQ-41-admin-dashboard`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** admin de una organización
**Quiero** ver, invitar, asignar roles, revocar miembros y transferir ownership desde una vista dedicada
**Para** gestionar el acceso de mi org sin tener que usar la API o el CLI

## Criterios de aceptación

```gherkin
Feature: Members Management

  Background:
    Given el usuario está autenticado con rol admin/owner
    And la org tiene al menos 1 miembro

  Scenario: Lista de miembros con tabla
    When navego a /admin/members
    Then veo una tabla con TODOS los members de la org:
      | columna     | fuente |
      | Avatar      | name initials |
      | Nombre      | user.name |
      | Email       | user.email |
      | Rol         | role.name |
      | Estado      | active / pending (invitación sin aceptar) |
      | Última actividad | last_sign_in_at (relativo) |
      | Acciones    | menú: cambiar rol, revocar, ver API keys |
    And la tabla tiene paginación (25/página), filtro por nombre/email, sort por columna

  Scenario: Invitar miembro vía email
    When hago clic en "Invitar miembro"
    Then se abre ModalComponent con form:
      | campo | validación |
      | email | required, formato email |
      | nombre | required, 1-100 chars |
      | rol | select con roles disponibles de la org |
    And al hacer submit, llama POST /api/v1/organizations/{id}/invitations
    And muestra toast "Invitación enviada a X"

  Scenario: Crear miembro con API key directa (sin email)
    When hago clic en "Crear miembro" (en lugar de invitar)
    Then se abre ModalComponent con form: email, name, role
    And al hacer submit, llama POST /api/v1/organizations/{id}/members (HU-36.1)
    And muestra modal de confirmación con la API key en plaintext (mostrar UNA sola vez)
    And un botón "Copiar" copia al clipboard + toast "Copiado"

  Scenario: Cambiar rol de un miembro
    When hago clic en "Cambiar rol" de un member
    Then se abre ModalComponent con FormSelectDirective listando roles
    And al confirmar, llama POST /api/v1/organizations/{id}/members/{user_id}/role
    And la fila se actualiza con el nuevo rol

  Scenario: Revocar invitación pendiente
    When hay invitaciones con status=pending
    Then veo una sección separada "Invitaciones pendientes" con tabla
    And cada fila tiene botón "Revocar"
    And al confirmar, llama POST /api/v1/invitations/{id}/revoke

  Scenario: Revocar miembro activo
    When hago clic en "Revocar" de un member activo
    Then se abre ModalComponent de confirmación "¿Revocar acceso de X?"
    And advierto que se invalidan sus API keys y sesiones
    And al confirmar, llama DELETE /api/v1/organizations/{id}/members/{user_id}
    And la fila pasa a estado "revoked" (o se elimina con confirmación)

  Scenario: Transferir ownership (solo owner)
    Given soy owner de la org
    When hago clic en "Transferir ownership" de un admin
    Then se abre ModalComponent de confirmación con doble check ("escribí TRANSFERIR")
    And advierto que el owner actual pasa a admin
    And al confirmar, llama POST /api/v1/organizations/{id}/transfer-ownership

  Scenario: Ver API keys de un miembro
    When hago clic en "Ver API keys" de un member
    Then se abre ModalComponent con tabla: name, prefix, created_at, last_used_at, status
    And cada fila tiene botón "Revocar" (DELETE /api/v1/api-keys/{id})
    And el plaintext de la key NO se muestra nunca (solo prefix + metadata)

  Scenario: Enrollment token (issue-37.1)
    Given la org tiene un enrollment token activo
    When hago clic en "Ver enrollment token"
    Then veo el token completo en plaintext + QR + botón "Rotar"
    And al rotar, llama POST /api/v1/organizations/{id}/enrollment-token/rotate
    And advierto que el token anterior deja de funcionar

  Scenario: Empty state
    Given la org no tiene members
    When veo /admin/members
    Then veo AlertComponent info "Esta org no tiene miembros. Empezá invitando a alguien."
    And el botón "Invitar miembro" está destacado
```

## Componentes del template CoreUI a reusar

| Componente | Path | Uso |
|---|---|---|
| `TableDirective` | `views/base/tables/` | Tabla principal de members + sub-tabla de invitaciones + sub-tabla de API keys |
| `PaginationComponent` | `views/base/paginations/` | Paginación de la tabla (25/página) |
| `ModalComponent` + header/body/footer/title + `ButtonCloseDirective` | `views/notifications/modal/` | Modales de invitar, crear, cambiar rol, revocar, transferir, ver API keys |
| `FormControlDirective` + `FormLabelDirective` | `views/forms/form-control/` | Inputs de email, name |
| `FormSelectDirective` | `views/forms/select/` | Selector de rol |
| `FormCheckComponent` | `views/forms/checks-radios/` | Checkbox "crear con API key directa" |
| `InputGroupComponent` | `views/forms/input-groups/` | Email con prefijo de dominio opcional |
| `ButtonDirective` | `views/buttons/` | Acciones de fila + CTAs |
| `ButtonGroup` | `views/buttons/button-groups/` | Acciones múltiples por fila |
| `BadgeComponent` | `views/notifications/badges/` | Status badge (active/pending/revoked) + role badge |
| `AlertComponent` | `views/notifications/alerts/` | Empty state, confirmación de warnings |
| `SpinnerComponent` | `views/base/spinners/` | Loading durante mutations |
| `ToasterComponent` + `ToastComponent` | `views/notifications/toasts/` | Feedback de éxito/error |
| `RowComponent` + `ColComponent` | `@coreui/angular` | Layout de modales y secciones |
| `IconDirective` (cil-*) | `@coreui/angular` | Iconos (`cil-people`, `cil-user-plus`, `cil-trash`, `cil-key`, `cil-shield-alt`) |

## Endpoints del backend

| Endpoint | Acción | Estado |
|---|---|---|
| `GET /api/v1/organizations/{id}/members` | Lista de members | ya existe |
| `POST /api/v1/organizations/{id}/members` | Crear member con API key directa | ya existe (issue-36.1) |
| `POST /api/v1/organizations/{id}/invitations` | Invitar por email | ya existe (issue-21.2) |
| `GET /api/v1/organizations/{id}/invitations` | Lista de invitaciones | ya existe |
| `POST /api/v1/invitations/{id}/revoke` | Revocar invitación | ya existe |
| `POST /api/v1/organizations/{id}/members/{user_id}/role` | Asignar/cambiar rol | ya existe (issue-02.8) |
| `DELETE /api/v1/organizations/{id}/members/{user_id}` | Revocar miembro | ya existe |
| `POST /api/v1/organizations/{id}/transfer-ownership` | Transferir ownership | ya existe (issue-21.1) |
| `GET /api/v1/api-keys?user_id=X` | API keys de un user | **VERIFICAR** si ya soporta filtro por user_id |
| `DELETE /api/v1/api-keys/{id}` | Revocar API key | ya existe |
| `GET /api/v1/organizations/{id}/enrollment-token` | Token activo | ya existe (issue-37.1) |
| `POST /api/v1/organizations/{id}/enrollment-token/rotate` | Rotar token | ya existe |

**Nuevos a crear en esta HU** (si hicieran falta):
- `GET /api/v1/users?org_id=X` (ya existe `/users`; verificar que soporta filtro por org) — **VERIFICAR primero**

## Análisis breve

- **Qué pide realmente:** Vista CRUD de members con sub-vistas de invitaciones y API keys. La pantalla más usada del admin.
- **Módulos a tocar:** Nueva vista `views/admin-members/`. Sin cambios de backend (asumiendo que los endpoints cubren todo).
- **Riesgos / dependencias:** Revocar un member es destructivo. Confirmar doble check. La transferencia de ownership solo la puede hacer el owner actual. El endpoint de API keys DEBE filtrar por user_id (validar primero).
- **Esfuerzo tentativo:** L

## Verificación previa

- [ ] Confirmar que `GET /api/v1/users` filtra por org correctamente
- [ ] Confirmar que `GET /api/v1/api-keys` filtra por user_id
- [ ] Verificar que `POST /members` con `role=owner` está bloqueado (solo transfer-ownership puede crear owner)
- [ ] Probar el flujo de "crear member con API key" (issue-36.1) y validar que el modal muestra la key UNA sola vez
- [ ] Confirmar que el endpoint de transfer-ownership requiere owner-only (no admin)
- [ ] Revisar si el enrollment token tiene un endpoint de "delete" además de "rotate" (issue-37.1)

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
