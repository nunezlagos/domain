# issue-41.7-admin-tickets-formalize

**Origen:** `REQ-41-admin-dashboard`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario con rol `developer` o superior
**Quiero** crear, asignar, comentar y dar seguimiento a tickets (issues internos) desde el panel admin
**Para** gestionar el trabajo operativo de la org sin abrir Jira/Linear ni usar el CLI

## Criterios de aceptación

```gherkin
Feature: Tickets (formalización)

  Background:
    Given el usuario está autenticado con rol developer/admin/owner
    And la org tiene al menos 1 proyecto

  Scenario: Lista de tickets (vista ya existe, refinar)
    When navego a /admin/tickets
    Then veo la tabla existente en views/tickets/tickets.component.ts con:
      | columna     | fuente |
      | Key         | display_key (e.g. "DOM-123") |
      | Title       | ticket.title |
      | Status      | badge (backlog/todo/in_progress/in_review/blocked/done/cancelled) |
      | Priority    | badge (trivial/low/medium/high/critical) |
      | Issue type  | bug/feature/task/chore |
      | Asignado    | user.name o "—" |
      | Lock        | icono candado si locked_by alguien |
      | Actualizado | relative timestamp |
    Y filtros: status, priority, assignee_id, project_slug
    Y paginación (25/página)

  Scenario: Crear ticket
    When hago clic en "Nuevo ticket"
    Then se abre ModalComponent con form:
      | campo | validación |
      | project | select (proyectos de la org) |
      | title | required, 3-200 chars |
      | description | textarea (markdown) |
      | issue_type | select (bug/feature/task/chore) |
      | priority | select (trivial/low/medium/high/critical) |
      | assignee | select con users de la org (opcional) |
      | labels | input con chips |
    Y al guardar, llama POST /api/v1/projects/{slug}/tickets
    Y redirige al detalle del ticket creado

  Scenario: Ver detalle de un ticket
    When hago clic en una fila
    Then navego a /admin/tickets/{key} con layout:
      | sección | contenido |
      | Header | key + title + status + priority + assignee + actions |
      | Description | markdown rendered |
      | Comments | thread de comments con author + timestamp + body |
      | History | timeline de cambios de status (status_history) |
      | Linked | links a issues/proyectos/tickets externos |

  Scenario: Cambiar status de un ticket
    When hago clic en "Cambiar status" en el detalle
    Then se abre ModalComponent con FormSelectDirective (status options)
    Y al confirmar, llama POST /api/v1/tickets/{id}/status
    Y se registra en status_history
    Y un comentario automático se postea en el thread

  Scenario: Reasignar ticket (ya implementado en tickets.component.ts)
    When hago clic en "Reasignar" de una fila
    Then se abre ModalComponent con FormSelectDirective de users
    Y al confirmar, llama PATCH /api/v1/tickets/{id} con {assignee_id}
    Y se registra en audit log

  Scenario: Lock/unlock de ticket
    Given un ticket está locked por mi user (porque lo estoy editando)
    When otro user intenta editar el mismo ticket
    Then el PUT/PATCH retorna 423 Locked
    Y la UI muestra AlertComponent warning "Este ticket está siendo editado por X. Esperá o contactalo."
    Y el lock expira automáticamente después de 5 min de inactividad

  Scenario: Comentar en ticket
    When escribo un comment en el thread
    Then al hacer submit, llama POST /api/v1/tickets/{id}/comments
    Y aparece inmediatamente en el thread
    Y notifica al assignee (si está configurado el canal)

  Scenario: Link a issue/proyecto interno
    When hago clic en "Link" en el detalle
    Then se abre ModalComponent con select (issue, proyecto, ticket)
    Y al confirmar, llama POST /api/v1/tickets/{id}/link-issue
    Y el link aparece en la sección "Linked"

  Scenario: Link externo (Jira, Linear)
    Given DOMAIN_JIRA_WEBHOOK_SECRET está configurado
    When linkeo a un issue de Jira (URL)
    Then llama POST /api/v1/tickets/{id}/link-external
    Y un badge "Jira" aparece junto al key
    Y el webhook de Jira actualiza el status (POST /api/v1/webhooks/jira/issue-updated)

  Scenario: Bulk actions
    When selecciono varios tickets (checkbox por fila)
    Then veo un ButtonGroup flotante con: Asignar, Cambiar status, Cerrar, Exportar
    Y la acción bulk llama al endpoint correspondiente con array de IDs

  Scenario: Filter "Mis tickets"
    When hago clic en tab "Mis tickets"
    Then veo solo tickets donde assignee_id = mi user_id
    Y un contador BadgeComponent con el total

  Scenario: Empty state
    Given la org no tiene tickets
    When veo /admin/tickets
    Then veo AlertComponent info "No hay tickets. Creá el primero con el botón de arriba."
```

## Componentes del template CoreUI a reusar

| Componente | Path | Uso |
|---|---|---|
| `TableDirective` | `views/base/tables/` | Tabla principal (ya existe en `views/tickets/`) |
| `PaginationComponent` | `views/base/paginations/` | Paginación de tickets |
| `Tabs` (Navs & Tabs) | `views/base/navs/` | Tabs "Todos / Mis tickets / Cerrados" |
| `ModalComponent` + set completo | `views/notifications/modal/` | Modales de crear, cambiar status, reasignar, lock, link |
| `FormControlDirective` + `FormLabelDirective` | `views/forms/form-control/` | Inputs de title, description, labels |
| `FormSelectDirective` | `views/forms/select/` | Selectores (project, status, priority, assignee, type) |
| `FormCheckComponent` | `views/forms/checks-radios/` | Checkbox por fila + selectAll en header |
| `InputGroupComponent` | `views/forms/input-groups/` | Search box con icono |
| `BadgeComponent` | `views/notifications/badges/` | Status + priority badges (ya hay `statusColor` y `priorityColor` en el código) |
| `ButtonDirective` | `views/buttons/` | Acciones de fila + CTAs |
| `ButtonGroup` | `views/buttons/button-groups/` | Bulk actions flotante |
| `AlertComponent` | `views/notifications/alerts/` | Empty state, warnings de lock |
| `SpinnerComponent` | `views/base/spinners/` | Loading |
| `ToasterComponent` | `views/notifications/toasts/` | Feedback de éxito/error |
| `CardComponent` + `CardBodyComponent` + `CardHeaderComponent` | `views/base/cards/` | Secciones del detalle (Description, Comments, History, Linked) |
| `ListGroupComponent` | `views/base/list-groups/` | Timeline de status_history |
| `IconDirective` (cil-*) | `@coreui/angular` | Iconos (`cil-list`, `cil-lock-locked`, `cil-link`, `cil-comment-square`, `cil-external-link`) |

**Componente existente a reusar y refinar**: `views/tickets/tickets.component.ts` ya tiene la tabla base. Esta HU lo formaliza, agrega el detalle (`views/tickets/ticket-detail/`), los modales de crear/editar, y los bulk actions.

## Endpoints del backend

| Endpoint | Acción | Estado |
|---|---|---|
| `GET /api/v1/tickets` | Lista con filtros y paginación | ya existe (issue-58) |
| `POST /api/v1/projects/{slug}/tickets` | Crear ticket en un proyecto | ya existe |
| `GET /api/v1/tickets/{id_or_key}` | Detalle de ticket | ya existe |
| `PATCH /api/v1/tickets/{id}` | Editar campos editables (incluye assignee, priority, etc.) | ya existe |
| `DELETE /api/v1/tickets/{id}` | Eliminar ticket | ya existe |
| `POST /api/v1/tickets/{id}/status` | Cambiar status (con history) | ya existe |
| `GET /api/v1/tickets/{id}/comments` | Lista de comments | ya existe |
| `POST /api/v1/tickets/{id}/comments` | Agregar comment | ya existe |
| `GET /api/v1/tickets/{id}/history` | Status history | ya existe |
| `POST /api/v1/tickets/{id}/link-external` | Link a issue externo | ya existe |
| `POST /api/v1/tickets/{id}/link-issue` | Link a issue/proyecto interno | ya existe |
| `POST /api/v1/tickets/link-external-bulk` | Bulk link (issue-58) | ya existe |
| `POST /api/v1/webhooks/jira/issue-updated` | Webhook Jira | ya existe (issue-58) |
| `GET /api/v1/users` | Lista de users para reasignar | ya existe (issue-75) |
| `GET /api/v1/projects` | Lista de proyectos para el form | ya existe |

**Nuevos a crear en esta HU** (si hicieran falta):
- `POST /api/v1/tickets/bulk/status` (bulk cambiar status) — **VERIFICAR** si existe. Si no, hacerlo como `POST /tickets/bulk-action` genérico.
- Lock TTL configurable por env (`DOMAIN_TICKET_LOCK_TTL=5m`) — **VERIFICAR** si ya está implementado.

## Análisis breve

- **Qué pide realmente:** Formalizar y extender lo que ya está en `views/tickets/`. La tabla base existe (commit `ff0e0d5`); esta HU agrega el detalle, el form de creación, los modales, bulk actions, y la integración con Jira.
- **Módulos a tocar:** Frontend: extender `views/tickets/` con `ticket-detail/`, `ticket-create-modal/`, y refinar `tickets.component.ts`. Backend: probablemente ninguno (los endpoints ya están).
- **Riesgos / dependencias:** El lock con TTL puede ser tricky — verificar el patrón actual. La integración con Jira requiere `DOMAIN_JIRA_WEBHOOK_SECRET` configurado.
- **Esfuerzo tentativo:** L

## Verificación previa

- [ ] Confirmar que `views/tickets/tickets.component.ts` ya funciona y reusarlo como base
- [ ] Verificar que el endpoint de lock funciona (¿qué status code retorna? ¿qué header?)
- [ ] Confirmar que `GET /api/v1/users` filtra por org (issue-75)
- [ ] Probar el flujo completo: crear → asignar → comentar → cambiar status → cerrar
- [ ] Verificar el caso Jira (con `DOMAIN_JIRA_WEBHOOK_SECRET` seteado)
- [ ] Decidir si el lock UI es solo "el último editor gana" o "el primero gana hasta TTL"

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
