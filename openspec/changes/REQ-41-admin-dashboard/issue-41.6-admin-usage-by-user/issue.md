# issue-41.6-admin-usage-by-user

**Origen:** `REQ-41-admin-dashboard`
**Prioridad tentativa:** alta
**Tipo:** feature

> **Cambio de scope (sesión 2026-06-16)**: el modelo es free total, sin planes ni billing. Esta HU pasa de "billing & usage" a "usage per-user". Sin upgrade, sin invoices, sin Stripe.

## Historia de usuario

**Como** admin de una organización
**Quiero** ver, para cada usuario de mi org, cuánto consume del servicio (prompts del mes, tokens input/output, runs ejecutados, storage usado, cost estimado)
**Para** identificar a los power users, detectar uso anómalo, optimizar costos internos, y tomar decisiones de capacity planning

## Criterios de aceptación

```gherkin
Feature: Usage by User (Admin View)

  Background:
    Given el usuario está autenticado con rol admin/owner
    And la org tiene al menos 3 miembros activos con actividad del mes

  Scenario: Tabla principal de usage por user
    When navego a /admin/usage
    Then veo una tabla con TODOS los miembros de la org + sus métricas del mes en curso:
      | columna           | fuente | sort |
      | User              | avatar + name + email | asc/desc |
      | Rol               | role.name (badge) | asc/desc |
      | Prompts (mes)     | count de captured_prompts del user este mes | numérico |
      | Tokens input      | sum token_usage.input_tokens del user este mes | numérico |
      | Tokens output     | sum token_usage.output_tokens del user este mes | numérico |
      | Runs              | count agent_runs + flow_runs del user este mes | numérico |
      | Storage MB        | size de attachments del user | numérico |
      | Cost estimado (USD) | sum token_usage.cost del user este mes | numérico |
      | Última actividad  | last_sign_in_at o last_token_usage.created_at | relativo |
    Y la tabla ordena por "Cost estimado DESC" por defecto
    Y la búsqueda por nombre/email filtra
    Y la paginación es 25/página

  Scenario: Filtros
    When aplico filtros
    Then veo FormSelectDirective para:
      | filtro | opciones |
      | Período | "Este mes" (default) / "Mes pasado" / "Últimos 30 días" / "Últimos 7 días" / "Personalizado" |
      | Orden | "Cost ↓" (default) / "Tokens ↓" / "Prompts ↓" / "Runs ↓" / "Nombre ↑" |
      | Solo activos | toggle (default ON: solo users con last_sign_in < 30d) |
    Y al cambiar, la tabla se recalcula y muestra un spinner mientras carga

  Scenario: Drill-down al detalle de un user
    When hago clic en una fila
    Then se abre ModalComponent "Detalle de uso de [user.name]" con:
      | sección | contenido |
      | Header | avatar + name + email + rol + fecha de ingreso |
      | Resumen mes | 4 stat cards: Prompts, Tokens total, Runs, Cost |
      | Gráfico temporal | line chart con cost diario del user (últimos 30 días) |
      | Top modelos | tabla con modelos usados, tokens, cost (% del total) |
      | Top proyectos | tabla con proyectos donde tuvo runs, count, cost |
      | Timeline | lista de últimas 10 acciones (run/completion) con timestamp |
    Y un botón "Ver audit de [user]" que abre /admin/audit?actor_id=X

  Scenario: Export CSV
    When hago clic en "Exportar CSV"
    Then descarga un .csv con TODAS las filas de la tabla (no solo la página actual)
    Y respeta el límite (default 5k rows; warning si excede)
    Y el CSV incluye las columnas: user_id, email, name, role, prompts, tokens_in, tokens_out, runs, storage_mb, cost_usd, last_activity_at

  Scenario: Sin datos del mes
    Given la org no tiene token_usage en el mes en curso
    When veo /admin/usage
    Then veo AlertComponent info "Aún no hay datos de uso del mes. La info aparece apenas los miembros ejecuten su primer agente o flow."
    Y la tabla muestra 0 en todas las métricas

  Scenario: User eliminado/desactivado
    Given un user fue revocado pero tiene token_usage histórica
    When veo la tabla
    Then el user aparece con badge "Revoked" + texto tachado
    Y sus métricas se siguen contando (no se borran)
    Y al hacer drill-down, un AlertComponent warning "Este user ya no es miembro activo"

  Scenario: super_admin cross-org
    Given soy super_admin
    When navego a /admin/usage
    Then veo un selector de org (super_admin only)
    Y puedo cambiar entre orgs y ver el usage de cada una
    Y el filtro "Período" sigue funcionando cross-org

  Scenario: Auto-refresh cada 60s
    When han pasado 60s
    Then la tabla reconsulta /api/v1/usage/by-user
    Y veo indicador "Actualizado hace Xs"
```

## Componentes del template CoreUI a reusar

| Componente | Path | Uso |
|---|---|---|
| `CardComponent` + `CardBodyComponent` + `CardHeaderComponent` | `views/base/cards/` | Header de página, stat cards en drill-down, wrapper de chart |
| `TableDirective` | `views/base/tables/` | Tabla principal de users + sub-tablas en drill-down |
| `PaginationComponent` | `views/base/paginations/` | Paginación (25/página) |
| `ModalComponent` + set completo | `views/notifications/modal/` | Modal de drill-down detalle de user |
| `FormControlDirective` + `FormLabelDirective` | `views/forms/form-control/` | Input de búsqueda |
| `FormSelectDirective` | `views/forms/select/` | Selectores (período, orden, org) |
| `FormCheckComponent` | `views/forms/checks-radios/` | Toggle "Solo activos" |
| `InputGroupComponent` | `views/forms/input-groups/` | Search box con icono |
| `BadgeComponent` | `views/notifications/badges/` | Role badge, status badge (Active/Revoked) |
| `ButtonDirective` | `views/buttons/` | Acciones (exportar, drill-down) |
| `AlertComponent` | `views/notifications/alerts/` | Empty state, warning de user revoked |
| `SpinnerComponent` | `views/base/spinners/` | Loading durante queries |
| `ToasterComponent` | `views/notifications/toasts/` | Feedback de export |
| `Tabs` (Navs & Tabs) | `views/base/navs/` | Tabs en drill-down (Resumen / Modelos / Proyectos / Timeline) |
| `ChartComponent` (line) | `views/charts/` | Line chart de cost diario por user |
| `IconDirective` (cil-*) | `@coreui/angular` | Iconos (`cil-user`, `cil-chart-line`, `cil-history`, `cil-dollar`, `cil-cloud-download`) |

## Endpoints del backend

| Endpoint | Acción | Estado |
|---|---|---|
| `GET /api/v1/usage/current` | Snapshot actual de usage del org | ya existe (issue-21.3) |
| `GET /api/v1/usage/history` | Histórico de usage del org | ya existe |
| `GET /api/v1/cost/breakdown/{dimension}` | Breakdown por agent/project/model/org | ya existe (issue-15.2) — **VERIFICAR** si acepta `dimension=user` |
| `GET /api/v1/captured-prompts?user_id=X` | Prompts capturados de un user | ya existe (REQ-41/47) |
| `GET /api/v1/agent-runs?user_id=X` | Runs de agente de un user | **VERIFICAR** filtro por user_id |
| `GET /api/v1/flow-runs?user_id=X` | Runs de flow de un user | **VERIFICAR** filtro por user_id |
| `GET /api/v1/users?org_id=X` | Lista de users de la org | ya existe (issue-75) |
| `GET /api/v1/audit-logs?actor_id=X` | Acciones de un user | ya existe (REQ-34.5) |

**Nuevos a crear en esta HU**:
- `GET /api/v1/usage/by-user` — **A CREAR**. Devuelve lista de users de la org con métricas agregadas del período. Server-side hace JOIN entre `users`, `token_usage`, `captured_prompts`, `agent_runs`, `flow_runs`. Filtros: `org_id` (requerido), `from`/`to` (opcional, default mes en curso), `only_active` (bool, default true). RBAC: `admin` solo ve SU org; `super_admin` puede pasar `?org_id=` para ver otra org. Respuesta:

  ```json
  {
    "period": { "from": "2026-06-01T00:00:00Z", "to": "2026-06-30T23:59:59Z" },
    "users": [
      {
        "user_id": "uuid",
        "email": "user@x.com",
        "name": "...",
        "role": "developer",
        "status": "active",
        "prompts": 142,
        "tokens_in": 123456,
        "tokens_out": 67890,
        "runs": 23,
        "storage_mb": 45.2,
        "cost_usd": 3.45,
        "last_activity_at": "2026-06-16T10:30:00Z"
      }
    ]
  }
  ```

  Server-side: 1 query agregada con `GROUP BY user_id` + `LEFT JOIN` a tabla `users`. Server-side hace el `errgroup` con paralelismo si la query es pesada.

- `GET /api/v1/usage/by-user/{user_id}` — **A CREAR**. Detalle drill-down de un user con breakdown por modelo y por proyecto. Similar a `/usage/by-user` pero devuelve la info de UN user con sub-breakdowns.

- `GET /api/v1/usage/by-user/export` — **A CREAR**. CSV stream con todas las filas. No carga en memoria (stream).

## Análisis breve

- **Qué pide realmente:** La vista central de observabilidad del admin. Lo más consultado día a día: "quién consume más?". Sin billing, sin plan, sin upgrade — solo métricas crudas por user.
- **Módulos a tocar:** Backend: `internal/api/handler/admin/usage_by_user_handler.go` con 3 endpoints nuevos. Frontend: nueva vista `views/admin-usage/`. Reutilizar los charts de HU-41.8 con un filtro user_id.
- **Riesgos / dependencias:** Performance crítica si la org tiene muchos users + mucho token_usage. Índices necesarios en `token_usage(user_id, created_at)`. Server-side debe hacer agregaciones con `GROUP BY` eficiente, no traer todas las rows. Caching opcional con TTL 30s (invalidar al recibir nuevo token_usage webhook).
- **Esfuerzo tentativo:** M (downgrade de L porque quitamos toda la parte de billing/Stripe)

## Verificación previa

- [ ] Confirmar que `token_usage` tiene índice en `user_id` (probablemente no, hay que migrar)
- [ ] Verificar si `GET /api/v1/usage/current` devuelve el desglose por user o solo agregado
- [ ] Confirmar RBAC: admin de Org A no puede ver `/usage/by-user?org_id=Org-B`
- [ ] Probar el caso "user con 0 actividad del mes" → ¿aparece en la tabla con métricas 0 o se omite?
- [ ] Decidir: ¿se muestra users sin actividad del mes? (probable: sí, con 0s)
- [ ] Decidir: ¿se incluye storage MB en las métricas por user? (atributo del user o de sus attachments)

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
