# issue-41.2-admin-org-dashboard

**Origen:** `REQ-41-admin-dashboard`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** admin de una organización
**Quiero** ver un dashboard resumen con stats de mi org, actividad reciente, system health y acciones rápidas
**Para** tener una visión general del estado de la plataforma al entrar al panel y poder saltar a las acciones comunes

## Criterios de aceptación

```gherkin
Feature: Org Dashboard (Admin Home)

  Background:
    Given el usuario está autenticado con rol admin/owner/super_admin
    And la org activa tiene al menos 1 proyecto

  Scenario: Dashboard carga con stat cards de la org
    When navego a /admin/dashboard
    Then veo 4 stat cards:
      | métrica           | fuente |
      | Miembros activos  | count members activos en org |
      | Agentes           | count agents en org |
      | Runs (últimas 24h)| count flow_runs + agent_runs en últimas 24h |
      | Tokens consumidos (mes) | sum tokens this month para org |
    And cada card tiene icono CoreUI y link a la vista detalle correspondiente

  Scenario: Widget de plan y uso
    When veo el dashboard
    Then veo el plan actual (Free/Pro/Enterprise) con barra de uso:
      | dimensión | usado / límite | barra |
      | Tokens/mes | X / Y | progress bar |
      | Runs/mes | X / Y | progress bar |
      | Storage GB | X / Y | progress bar |
      | Members | X / Y | progress bar |
    And si alguna dimensión supera 80%, la barra se pone amarilla; si supera 100%, roja

  Scenario: Widget de actividad reciente
    When veo el dashboard
    Then veo las últimas 10 acciones del audit_log de la org
    And cada entry muestra: actor, action, target, timestamp relativo ("hace 5m")
    And un link "Ver todo" lleva a /admin/audit

  Scenario: Widget de system health (super_admin only)
    Given mi rol activo es super_admin
    When veo el dashboard
    Then veo 3 status cards con health del sistema:
      | componente     | fuente |
      | API Backend    | GET /api/v1/healthz |
      | Database       | GET /api/v1/admin/db-stats (procesar err) |
      | LLM Providers  | endpoint a definir o derivar de /api/v1/admin/runtime-configs |
    And cada card tiene color: verde/amarillo/rojo + tooltip con detalle

  Scenario: Acciones rápidas
    When veo el dashboard
    Then veo 4 botones de acción rápida:
      | acción              | comportamiento |
      | Invitar miembro     | abre modal con form de invitación |
      | Crear proyecto      | navega a /admin/projects/new |
      | Crear agente        | navega a /admin/agents/new |
      | Ver tickets abiertos| navega a /admin/tickets?status=todo |
    And cada botón tiene icono CoreUI y label claro

  Scenario: Auto-refresh cada 30s
    When han pasado 30s desde la última carga
    Then el dashboard reconsulta /api/v1/admin/org-overview
    And veo indicador "Actualizado hace Xs"

  Scenario: Empty state
    Given la org no tiene proyectos/agentes/miembros
    When veo el dashboard
    Then veo mensaje "Aún no hay actividad. Empezá creando un proyecto o invitando un miembro."
    And los botones de acción rápida están destacados

  Scenario: Error al cargar
    Given /api/v1/admin/org-overview retorna 500
    When veo el dashboard
    Then veo AlertComponent danger "No pudimos cargar el dashboard. Reintentá."
    And un botón "Reintentar"
```

## Componentes del template CoreUI a reusar

| Componente | Path | Uso |
|---|---|---|
| `CardComponent` + `CardBodyComponent` + `CardHeaderComponent` | `views/base/cards/` | Stat cards, plan/uso widget, activity widget, health widget, acciones rápidas |
| `ProgressComponent` (progress bar) | `views/base/progress/` | Barras de uso del plan (con color variant success/warning/danger) |
| `BadgeComponent` | `views/notifications/badges/` | Status badges en health cards |
| `ButtonDirective` | `views/buttons/` | Acciones rápidas (variant=primary, outline, etc.) |
| `AlertComponent` | `views/notifications/alerts/` | Empty state + error state |
| `SpinnerComponent` | `views/base/spinners/` | Loading mientras carga `org-overview` |
| `PlaceholderAnimationDirective` | `views/base/placeholders/` | Skeleton mientras carga |
| `RowComponent` + `ColComponent` | `@coreui/angular` | Grid responsive de los widgets |
| `IconDirective` (cil-*) | `@coreui/angular` | Iconos en stat cards y acciones (`cil-people`, `cil-robot`, `cil-sprint`, `cil-dollar`) |
| Patrón HttpClient+signals | `views/tickets/tickets.component.ts` | Referencia de cómo estructurar data fetching |

## Endpoints del backend

| Endpoint | Acción | Estado |
|---|---|---|
| `GET /api/v1/admin/org-overview` | Devuelve todo lo del dashboard en 1 request | **A CREAR** en esta HU |
| `GET /api/v1/healthz` | Health del API | ya existe |
| `GET /api/v1/admin/db-stats` | Stats de DB | ya existe (RE-25.12) |

**Endpoint nuevo** `GET /api/v1/admin/org-overview` debe devolver:

```json
{
  "stats": {
    "members_active": 12,
    "agents": 8,
    "runs_24h": 143,
    "tokens_this_month": 1234567
  },
  "plan": {
    "name": "pro",
    "limits": { "tokens_per_month": 5000000, "runs_per_month": 10000, "storage_gb": 50, "members": 25 },
    "usage": { "tokens_per_month": 1234567, "runs_per_month": 4521, "storage_gb": 12, "members": 12 }
  },
  "recent_activity": [
    { "actor": "user@x.com", "action": "member.invited", "target": "user@y.com", "at": "2026-06-16T10:30:00Z" }
  ],
  "system_health": {
    "api": "ok",
    "database": "ok",
    "llm_providers": "ok"
  }
}
```

- RBAC: `admin` solo ve SU org; `super_admin` puede pasar `?org_id=` para ver otra org.
- Server-side agrega queries en paralelo con `errgroup` (patrón del issue-16.1 original).

## Análisis breve

- **Qué pide realmente:** Vista home del panel admin. Es lo primero que ve un admin al entrar. Reemplaza el dashboard de stock de CoreUI (que es una demo).
- **Módulos a tocar:** Backend: `internal/api/handler/admin/org_overview_handler.go`. Frontend: nueva vista `views/admin-dashboard/`.
- **Riesgos / dependencias:** Si el endpoint `/admin/org-overview` se hace lento con muchas queries, agregar caching server-side (TTL 15s, invalidar al recibir webhook de audit). El super_admin cross-org switcher depende de HU-41.1.
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Confirmar que `/api/v1/usage` (HU 21.3) devuelve los datos de plan/uso que necesitamos
- [ ] Confirmar que `/api/v1/audit-logs?limit=10` funciona
- [ ] Verificar que `db-stats` se puede llamar desde el dashboard (RBAC super_admin only)
- [ ] Confirmar patrón de `errgroup` en handlers existentes (issue-16.1 usa uno)
- [ ] Verificar que el stats widget no rompe si una org no tiene plan asignado (default Free)

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
