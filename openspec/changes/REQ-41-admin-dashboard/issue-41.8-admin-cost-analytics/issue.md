# issue-41.8-admin-cost-analytics

**Origen:** `REQ-41-admin-dashboard`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** admin de una organización
**Quiero** ver gráficos de cost (gasto por día/semana/mes, breakdown por agente/proyecto, forecast del mes) y definir budgets con alertas
**Para** entender dónde se va la plata, prevenir excesos y tomar decisiones de optimización

## Criterios de aceptación

```gherkin
Feature: Cost Analytics

  Background:
    Given el usuario está autenticado con rol admin/owner
    And la org tiene al menos 30 días de cost data

  Scenario: Vista de cost analytics con tabs
    When navego a /admin/cost
    Then veo la página con Tabs:
      | tab | contenido |
      | Overview | Cost summary + line chart de los últimos 30 días |
      | Breakdown | Stacked bar chart de cost por dimensión (agent/project/model/**user**) |
      | Forecast | Proyección del mes en curso + próximos 30 días |
      | Budgets | Lista de budgets configurados + creación |

  Scenario: Cost summary (Overview)
    When veo la tab Overview
    Then veo 4 stat cards (con CardComponent):
      | métrica | valor |
      | Costo hoy | $X.XX |
      | Costo este mes | $X.XX (vs $Y.YY mes pasado) |
      | Costo promedio/día | $X.XX (últimos 30 días) |
      | Costo proyectado fin de mes | $X.XX (basado en uso al día de hoy) |
    Y un line chart con cost diario de los últimos 30 días (ChartComponent de CoreUI)

  Scenario: Breakdown por dimensión
    When veo la tab Breakdown
    Then veo un FormSelectDirective para elegir dimensión: agent / project / model / org / **user**
    Y un stacked bar chart con el top 10 de la dimensión elegida
    Y una tabla con el detalle completo (top 50)
    Y columnas: nombre, cost total, % del total, trend (vs período anterior)
    Y si la dimensión es "user", cada fila muestra name + email + link al drill-down de usage (HU-41.6)

  Scenario: Forecast
    When veo la tab Forecast
    Then veo un line chart con:
      | línea | descripción |
      | Real | cost diario histórico (últimos 30 días) |
      | Proyectado | cost diario futuro (próximos 30 días) |
      | Budget | línea horizontal del budget configurado |
    Y un AlertComponent warning si la proyección supera el budget

  Scenario: Crear budget
    When hago clic en "Nuevo budget" en tab Budgets
    Then se abre ModalComponent con form:
      | campo | opciones |
      | Nombre | text |
      | Dimensión | select (tokens, runs, cost_usd) |
      | Límite | número |
      | Período | select (daily/weekly/monthly) |
      | Alerta al | número (% del límite, default 80) |
    Y al guardar, llama POST /api/v1/cost/budgets
    Y se crea una usage_alert asociada (issue-21.3)

  Scenario: Budgets existentes
    When hay budgets configurados
    Then veo una tabla con: nombre, dimensión, límite, uso actual (% con ProgressComponent), período, status
    Y cada fila tiene botones: Editar, Pausar, Eliminar

  Scenario: Filtros de rango
    When elijo rango de fechas (date range picker, default últimos 30 días)
    Then todos los charts y tablas se recalculan para el rango
    Y el breakdown se actualiza con sort por cost total DESC

  Scenario: Export CSV
    When hago clic en "Exportar"
    Then descarga un .csv con el breakdown completo del rango seleccionado
    Y respeta el límite de export (default 50k rows)

  Scenario: Sin datos
    Given la org no tiene cost data
    When veo /admin/cost
    Then veo AlertComponent info "Aún no hay datos de cost. Empezá a usar agentes/flows para generar actividad."
    Y los charts muestran empty state con icono y mensaje
```

## Componentes del template CoreUI a reusar

| Componente | Path | Uso |
|---|---|---|
| `CardComponent` + `CardBodyComponent` + `CardHeaderComponent` | `views/base/cards/` | Stat cards, wrappers de charts, budget cards |
| `Tabs` (Navs & Tabs) | `views/base/navs/` | Tabs Overview / Breakdown / Forecast / Budgets |
| `ProgressComponent` | `views/base/progress/` | Barras de uso de budgets (variant success/warning/danger) |
| `TableDirective` | `views/base/tables/` | Tabla de breakdown, tabla de budgets |
| `ModalComponent` + set completo | `views/notifications/modal/` | Modal de crear/editar budget |
| `FormControlDirective` + `FormLabelDirective` | `views/forms/form-control/` | Inputs (nombre, límite) |
| `FormSelectDirective` | `views/forms/select/` | Selectores (dimensión, período) |
| `ButtonDirective` | `views/buttons/` | Acciones (nuevo budget, exportar) |
| `BadgeComponent` | `views/notifications/badges/` | Status badges (Activo/Pausado/Excedido) |
| `AlertComponent` | `views/notifications/alerts/` | Empty state, warning de budget excedido |
| `SpinnerComponent` | `views/base/spinners/` | Loading de queries |
| `ToasterComponent` | `views/notifications/toasts/` | Feedback de éxito/error |
| `ChartComponent` (line + bar + stacked) | `views/charts/` | Todos los gráficos de cost |
| `PaginationComponent` | `views/base/paginations/` | Paginación de tabla de breakdown (50/página) |
| `IconDirective` (cil-*) | `@coreui/angular` | Iconos (`cil-dollar`, `cil-chart-line`, `cil-target`, `cil-calendar`) |

## Endpoints del backend

| Endpoint | Acción | Estado |
|---|---|---|
| `GET /api/v1/cost/daily` | Cost diario (últimos N días) | ya existe (issue-15.1) |
| `GET /api/v1/cost/spend/{granularity}` | Cost por día/semana/mes | ya existe (issue-15.1) |
| `GET /api/v1/cost/breakdown/{dimension}` | Breakdown por agent/project/model | ya existe (issue-15.2) |
| `GET /api/v1/cost/forecast` | Proyección de cost | ya existe (issue-15.3) |
| `GET /api/v1/cost/budgets` | Lista de budgets | ya existe (issue-15.3) |
| `POST /api/v1/cost/budgets` | Crear budget | ya existe (issue-15.3) |
| `DELETE /api/v1/cost/budgets/{id}` | Eliminar budget | ya existe (issue-15.3) |
| `GET /api/v1/cost/export` | Export CSV | ya existe (issue-15.3) |
| `GET /api/v1/usage-alerts` | Alertas de budget (vinculadas) | ya existe (issue-21.3) |

**Nuevos a crear en esta HU** (si hicieran falta):
- `PATCH /api/v1/cost/budgets/{id}` (editar/pausar budget) — **VERIFICAR** si existe. Si no, agregarlo.

## Análisis breve

- **Qué pide realmente:** Vista de cost analytics con charts y budgets. Toda la data ya está disponible en el backend (REQ-15 + REQ-21.3). Esta HU es puro frontend, salvo el endpoint de edit de budget.
- **Módulos a tocar:** Nueva vista `views/admin-cost/`. Backend: posiblemente agregar `PATCH /cost/budgets/{id}`.
- **Riesgos / dependencias:** Los charts de CoreUI Free son limitados (basic line/bar). Si necesitamos stacked/area, evaluar si CoreUI PRO lo cubre o usar una lib externa (Chart.js directo). Decisión: empezar con CoreUI Free; si limita, migrar a Chart.js después.
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Confirmar que `GET /api/v1/cost/breakdown/agent` y `/project` funcionan
- [ ] Verificar que `cost/forecast` devuelve el horizonte esperado (30 días?)
- [ ] Confirmar que los budgets tienen endpoint de edit (no solo create/delete)
- [ ] Decidir: ¿CoreUI Free charts o migrar a Chart.js?
- [ ] Validar el caso "org con 0 cost data" (no debe romper la UI)
- [ ] Confirmar el formato del export CSV (columnas esperadas)

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
