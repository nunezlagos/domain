# issue-41.9-admin-super-admin-cross-org

**Origen:** `REQ-41-admin-dashboard`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** super_admin de la plataforma
**Quiero** ver métricas cross-org, un listado de TODAS las orgs, switchear entre ellas, y ver system health de la plataforma
**Para** operar el SaaS: detectar problemas, hacer onboarding, y dar soporte

## Criterios de aceptación

```gherkin
Feature: Super Admin Cross-Org

  Background:
    Given el usuario está autenticado con rol super_admin
    And la plataforma tiene al menos 2 orgs

  Scenario: Sidebar muestra item "Cross-org" (solo super_admin)
    Given soy super_admin
    When navego a /admin/dashboard
    Then veo un item "Cross-org" en el sidebar (separado del nav de admin)
    And el item abre /admin/cross-org

  Scenario: Vista cross-org con métricas globales
    When navego a /admin/cross-org
    Then veo una vista con 4 stat cards (cross-org):
      | métrica              | fuente |
      | Total de orgs        | count organizations |
      | Total de users       | count users activos (todos los tenants) |
      | Cost total plataforma mes | sum cost_this_month cross-org |
      | Runs plataforma hoy  | count runs cross-org últimas 24h |
    Y un AlertComponent info "Vista cross-org: estas métricas incluyen TODAS las orgs. Usala con cuidado."

  Scenario: Tabla de orgs con sortable y filter
    When veo la sección "Organizaciones"
    Then veo una tabla con:
      | columna    | descripción |
      | Name       | org.name + logo (si branding) |
      | Slug       | org.slug (link copiable) |
      | Plan       | plan actual + badge |
      | Members    | count members activos |
      | Cost (mes) | $X.XX + sparkline 7d |
      | Status     | active / suspended / deleted |
      | Creada     | created_at relativo |
      | Acciones   | menú: Ver, Entrar (impersonar), Suspender, Eliminar |
    Y filtros: name, plan, status, rango creación
    Y sort por cualquier columna
    Y paginación (50/página)

  Scenario: Entrar a una org (impersonation)
    When hago clic en "Entrar como admin" de una org
    Then se abre ModalComponent de confirmación:
      | campo | contenido |
      | Warning | "Vas a entrar como owner de esta org. TODAS tus acciones quedan registradas con tu identidad real en el audit log." |
      | Checkbox | "Entiendo que esto se registra para compliance" |
      | Botón | "Entrar" (disabled hasta check) |
    Y al confirmar, llama POST /api/v1/admin/impersonate (HU-41.10)
    Y redirige a /admin/dashboard con el org context cambiado
    Y aparece un banner amarillo sticky: "Estás impersonando a Org X. Click acá para salir."

  Scenario: Suspender una org
    When hago clic en "Suspender" de una org
    Then se abre ModalComponent con razón (textarea, required)
    Y advierto: "Los usuarios no podrán loguearse. Los runs en curso se cancelan."
    Y al confirmar, llama PATCH /api/v1/clients/{id}/status con {status: suspended}
    Y la org aparece con badge "Suspended" en la tabla

  Scenario: System health (super_admin only)
    When hago clic en "System health" en /admin/cross-org
    Then veo una vista con status de todos los componentes:
      | componente           | fuente | status |
      | API Backend          | GET /api/v1/healthz | ok/error |
      | Database             | GET /api/v1/admin/db-stats (procesar) | ok/warning/error |
      | Object storage (MinIO)| health check interno | ok/error |
      | LLM Providers        | listado de providers + last_check_at | ok/error per provider |
      | Background workers   | count workers activos | ok/error |
      | Disk usage           | % uso del disco de la VPS | ok/warning (>80%)/error (>90%) |
    Y cada componente tiene color verde/amarillo/rojo
    Y un AlertComponent danger si algo está en rojo con CTA "Ver logs"

  Scenario: Crear org nueva (super_admin only)
    When hago clic en "Nueva org"
    Then se abre ModalComponent con form:
      | campo | validación |
      | name | required, 1-100 chars |
      | slug | required, lowercase + guiones, único |
      | plan | select (free/pro/enterprise) |
      | owner_email | required, email válido |
    Y al guardar, llama POST /api/v1/organizations
    Y se crea el owner user con API key inicial (mostrar UNA sola vez)
    Y la org aparece en la tabla

  Scenario: Auditoría de acciones cross-org
    Given soy super_admin
    When suspendo una org o entro como owner
    Then la acción se registra en audit log con `actor_type=super_admin` + `actor_id` + `target_org_id`
    Y puedo ver estas acciones en /admin/audit con filtro actor_type=super_admin

  Scenario: Cambio de org context
    Given estoy impersonando Org A
    When hago clic en "Salir de impersonation" en el banner
    Then llama POST /api/v1/admin/impersonate/stop
    Y restaura la sesión de super_admin
    Y el header vuelve a mostrar "Todas las orgs" o el org_id del super_admin

  Scenario: Sin ser super_admin
    Given soy admin (no super_admin)
    When navego a /admin/cross-org
    Then el authGuard de la ruta redirige a /admin/dashboard
    Y el item "Cross-org" no aparece en el sidebar
```

## Componentes del template CoreUI a reusar

| Componente | Path | Uso |
|---|---|---|
| `CardComponent` + `CardBodyComponent` + `CardHeaderComponent` | `views/base/cards/` | Stat cards cross-org, system health cards, secciones |
| `TableDirective` | `views/base/tables/` | Tabla de orgs (sortable + filter) |
| `PaginationComponent` | `views/base/paginations/` | Paginación de orgs (50/página) |
| `BadgeComponent` | `views/notifications/badges/` | Plan badges, status badges (active/suspended) |
| `ModalComponent` + set completo | `views/notifications/modal/` | Modales de impersonar, suspender, crear org, ver health detail |
| `FormControlDirective` + `FormLabelDirective` | `views/forms/form-control/` | Inputs de crear org |
| `FormSelectDirective` | `views/forms/select/` | Selectores (plan, status, filtros) |
| `FormCheckComponent` | `views/forms/checks-radios/` | Checkbox de "entiendo" en modal de impersonar |
| `InputGroupComponent` | `views/forms/input-groups/` | Filtros con iconos |
| `ButtonDirective` | `views/buttons/` | Acciones (nueva org, suspender, impersonar) |
| `ButtonGroup` | `views/buttons/button-groups/` | Toggle "Activas / Suspendidas / Todas" |
| `AlertComponent` | `views/notifications/alerts/` | Warnings de impersonation, danger de system health |
| `SpinnerComponent` | `views/base/spinners/` | Loading |
| `ToasterComponent` | `views/notifications/toasts/` | Feedback |
| `Tabs` (Navs & Tabs) | `views/base/navs/` | Tabs "Orgs / System health / Auditoría cross-org" |
| `IconDirective` (cil-*) | `@coreui/angular` | Iconos (`cil-globe-alt`, `cil-building`, `cil-shield-alt`, `cil-heart`, `cil-warning`) |
| Sticky banner (custom sobre `AlertComponent`) | `views/notifications/alerts/` | Banner de "Estás impersonando" (variant warning, dismissable) |

## Endpoints del backend

| Endpoint | Acción | Estado |
|---|---|---|
| `GET /api/v1/organizations` | Lista de orgs (super_admin ve todas, admin ve solo la suya) | ya existe |
| `POST /api/v1/organizations` | Crear org | ya existe |
| `PATCH /api/v1/clients/{id}/status` | Cambiar status de org (active/suspended) | ya existe |
| `GET /api/v1/clients/{id_or_slug}` | Detalle de org | ya existe |
| `GET /api/v1/admin/db-stats` | Stats de DB | ya existe (RE-25.12) |
| `GET /api/v1/admin/db/slow-queries` | Slow queries | ya existe (RE-25.11) |
| `GET /api/v1/admin/runtime-configs/{key}` | Config de plataforma | ya existe |
| `GET /api/v1/audit-logs?actor_type=super_admin` | Audit de acciones cross-org | ya existe (REQ-34.5) |
| `POST /api/v1/admin/impersonate` | Entrar como otro user (HU-41.10) | **A CREAR** en HU-41.10 |
| `POST /api/v1/admin/impersonate/stop` | Salir de impersonation (HU-41.10) | **A CREAR** en HU-41.10 |
| `GET /api/v1/admin/impersonate/active` | Estado de impersonation (HU-41.10) | **A CREAR** en HU-41.10 |

**Nuevos a crear en esta HU**:
- `GET /api/v1/admin/cross-org-stats` → métricas agregadas cross-org (orgs, users, cost, runs). **A CREAR**.
- `GET /api/v1/admin/system-health` → health completo (API + DB + LLM providers + workers + disk). **A CREAR**.
- `GET /api/v1/admin/disk-usage` → % uso del disco (o derivar de `db-stats`). Evaluar.

## Análisis breve

- **Qué pide realmente:** El "god mode" del SaaS. Solo accesible para super_admin. Combina: lista de orgs, métricas cross-org, system health, impersonation, y acciones administrativas cross-org (suspender, crear).
- **Módulos a tocar:** Frontend: nueva vista `views/admin-cross-org/`. Backend: `internal/api/handler/admin/cross_org_stats.go`, `system_health.go`, `disk_usage.go`. El super_admin auth check se hace en el middleware existente (rol-based).
- **Riesgos / dependencias:** Esta vista tiene el MÁXIMO privilegio. Hay que ser estrictos con el RBAC (super_admin only). El endpoint `/admin/cross-org-stats` NO debe ser accesible por admin normal. El impersonation (HU-41.10) es prerequisito para "Entrar como admin".
- **Esfuerzo tentativo:** L

## Verificación previa

- [ ] Confirmar que el middleware de auth distingue super_admin de admin
- [ ] Verificar que `GET /api/v1/organizations` SIN filtro devuelve todas las orgs para super_admin y solo la propia para admin
- [ ] Confirmar que `PATCH /api/v1/clients/{id}/status` funciona y registra audit
- [ ] Verificar si `db-stats` ya da información suficiente para system health o hay que agregar endpoint dedicado
- [ ] Decidir: ¿`system-health` es 1 endpoint o varios (uno por componente)?
- [ ] Confirmar que el audit log soporta filtro por `actor_type=super_admin` (REQ-34.5)

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
