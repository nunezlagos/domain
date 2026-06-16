# issue-41.5-admin-audit-log-viewer

**Origen:** `REQ-41-admin-dashboard`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** admin de una organización (o super_admin cross-org)
**Quiero** ver y filtrar el audit log de acciones ocurridas en mi org (con actor, acción, recurso, timestamp)
**Para** detectar accesos sospechosos, investigar incidentes y demostrar compliance

## Criterios de aceptación

```gherkin
Feature: Audit Log Viewer

  Background:
    Given el usuario está autenticado con rol admin/owner/super_admin
    And la org tiene al menos 100 eventos en audit_log

  Scenario: Tabla de audit con filtros
    When navego a /admin/audit
    Then veo una tabla con columnas: timestamp, actor, action, target, org_id (si super_admin), IP
    Y arriba, una barra de filtros:
      | filtro          | tipo |
      | Actor (email)   | input text con autocomplete |
      | Action          | select con acciones pre-definidas (member.invited, role.assigned, project.created, etc.) |
      | Recurso (slug)  | input text |
      | Desde / Hasta   | date range picker (default últimos 7 días) |
      | Org (solo super_admin) | select con todas las orgs |

  Scenario: Paginación y orden
    When hay >50 resultados
    Then veo PaginationComponent abajo (50/página, default)
    Y la tabla ordena por timestamp DESC por defecto
    Y se puede cambiar el sort por cualquier columna

  Scenario: Ver detalle de un evento
    When hago clic en una fila
    Then se abre ModalComponent con:
      | campo | descripción |
      | ID | uuid del evento |
      | Actor | user + email + role |
      | Action | nombre + descripción legible |
      | Target | tipo + id + nombre |
      | IP | ip de origen |
      | User-Agent | navegador/cliente |
      | Diff (JSON) | before/after state si aplica |
      | Timestamp | ISO 8601 + relativo |
    Y un botón "Copiar JSON"

  Scenario: Exportar a CSV
    When hago clic en "Exportar CSV"
    Then descarga un .csv con los filtros aplicados
    Y respeta el límite de export (default 10k rows; warning si excede)

  Scenario: Búsqueda full-text
    When escribo "api_key" en el filtro de recurso
    Then veo solo eventos cuyo target contenga "api_key"
    Y la búsqueda es case-insensitive y por substring

  Scenario: Vista de admin solo ve SU org
    Given soy admin de Org A
    When navego a /admin/audit
    Then veo solo eventos de Org A
    Y no puedo seleccionar otra org (filtro de org no aparece)

  Scenario: super_admin ve TODAS las orgs
    Given soy super_admin
    When navego a /admin/audit
    Then veo el filtro de org con todas las orgs disponibles
    Y el filtro por defecto es "Todas las orgs"
    Y puedo filtrar por org específica

  Scenario: Real-time refresh (opcional)
    When hay un nuevo evento relevante
    Then aparece un BadgeComponent en la tab con count de eventos no leídos
    Y se actualiza cada 30s (polling) o vía SSE (issue-69 /events)

  Scenario: Empty state
    Given no hay eventos en el rango
    When veo /admin/audit
    Then veo AlertComponent info "No hay eventos en este rango. Ampliá las fechas o limpia los filtros."
```

## Componentes del template CoreUI a reusar

| Componente | Path | Uso |
|---|---|---|
| `TableDirective` | `views/base/tables/` | Tabla principal de eventos |
| `PaginationComponent` | `views/base/paginations/` | Paginación (50/página) |
| `ModalComponent` + set completo | `views/notifications/modal/` | Modal de detalle de evento |
| `FormControlDirective` + `FormLabelDirective` | `views/forms/form-control/` | Inputs de filtros (actor, recurso) |
| `FormSelectDirective` | `views/forms/select/` | Selectores (action, org) |
| `DateRange` (custom sobre `form-control`) | `views/forms/form-control/` | Date range desde/hasta (CoreUI Free no tiene nativo, armar custom) |
| `InputGroupComponent` | `views/forms/input-groups/` | Filtros con iconos |
| `BadgeComponent` | `views/notifications/badges/` | Count de eventos no leídos, color por tipo de action |
| `ButtonDirective` | `views/buttons/` | Acciones (exportar CSV, refresh) |
| `ButtonGroup` | `views/buttons/button-groups/` | Toggle "Filtros avanzados" |
| `AlertComponent` | `views/notifications/alerts/` | Empty state, info de "X resultados" |
| `SpinnerComponent` | `views/base/spinners/` | Loading de queries |
| `ToasterComponent` | `views/notifications/toasts/` | Feedback de export |
| `Tabs` | `views/base/tabs/` | Si separamos "Todos" / "Sospechosos" / "Míos" |
| `IconDirective` (cil-*) | `@coreui/angular` | Iconos (`cil-history`, `cil-filter`, `cil-download`, `cil-user`) |
| Patrón HttpClient + signals | `views/tickets/tickets.component.ts` | Data fetching y paginación |

## Endpoints del backend

| Endpoint | Acción | Estado |
|---|---|---|
| `GET /api/v1/audit-logs` | Lista de eventos con filtros y paginación | ya existe (issue-02.4) |
| `GET /api/v1/audit-logs/{id}` | Detalle de un evento | **VERIFICAR**; si no existe, devolver el mismo listado filtrado por id |
| `GET /api/v1/audit-logs/export` | Export CSV | **VERIFICAR**; si no existe, armar con los mismos filtros + `?format=csv` |
| `GET /api/v1/events` (SSE) | Stream de eventos nuevos | ya existe (issue-69) |

**Verificaciones críticas** (REQ-34.5 ya cubre multi-tenant para audit):
- admin de Org A NO puede ver eventos de Org B (filtro por `org_id` en backend)
- super_admin con `?org_id=` específico ve solo esa org; sin `?org_id=` ve todas

**Nuevos a crear en esta HU** (si hicieran falta):
- `GET /api/v1/audit-logs/{id}/detail` con `before_state`, `after_state`, `diff` — **VERIFICAR** si el listado ya devuelve estos campos o hay que agregar endpoint

## Análisis breve

- **Qué pide realmente:** Vista de solo-lectura del audit log con filtros potentes, paginación, export, y vista detalle. El admin de seguridad (compliance) la va a usar a diario.
- **Módulos a tocar:** Nueva vista `views/admin-audit/`. Backend: posiblemente extender `audit_handler.go` con endpoint de detalle y export CSV. Req-34.5 ya cubre multi-tenant.
- **Riesgos / dependencias:** El audit log puede tener millones de rows. Backend DEBE tener índices en `(org_id, created_at DESC)` y `(actor_id, created_at DESC)`. Performance crítica. El export CSV no debe cargar todo en memoria (streaming).
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Confirmar que el `GET /api/v1/audit-logs` soporta todos los filtros que necesitamos
- [ ] Verificar índices en `audit_log` (org_id, created_at, actor_id)
- [ ] Confirmar que `REQ-34.5` ya implementó el filtro multi-tenant estricto
- [ ] Probar el caso "admin de Org A intenta ver audit de Org B" → debe recibir 403 o 404
- [ ] Confirmar que el SSE `/api/v1/events` emite eventos de audit (no solo de runs)
- [ ] Decidir límite del export CSV (10k? 100k? streaming infinito?)

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
