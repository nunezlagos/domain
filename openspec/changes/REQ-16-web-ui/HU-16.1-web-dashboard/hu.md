# HU-16.1-web-dashboard

**Origen:** `REQ-16-web-ui`
**Persona:** org-member, org-admin
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** administrador de la Plataforma Domain
**Quiero** un dashboard web con visión general del sistema: stats, actividad reciente, acciones rápidas y resumen de costos
**Para** monitorear el estado del sistema y acceder rápidamente a las funcionalidades principales

## Criterios de aceptación

```gherkin
Feature: Web Dashboard

  Background:
    Given el servidor web está corriendo en http://localhost:3000
    And el usuario está autenticado con rol admin

  Scenario: Dashboard carga con stats generales
    When navego a /dashboard
    Then veo tarjetas de estadísticas:
      | métrica          | descripción                |
      | Total Agents     | número de agentes activos  |
      | Total Flows      | número de flows definidos  |
      | Total Skills     | número de skills           |
      | Total Runs       | número de ejecuciones hoy  |
    And cada tarjeta tiene un icono representativo

  Scenario: Recent activity feed
    When veo el dashboard
    Then veo una lista de actividad reciente (últimas 20 acciones)
    And cada entrada muestra: tipo, descripción, timestamp relativo ("hace 5m")
    And las entradas están ordenadas por fecha descendente

  Scenario: Cost summary card
    When veo el dashboard
    Then veo un resumen de costos:
      | métrico          | valor                     |
      | Costo hoy        | $X.XX                     |
      | Costo este mes   | $X.XX                     |
      | Costo promedio/día| $X.XX                     |
    And un mini gráfico de barras de los últimos 7 días

  Scenario: Quick actions
    When veo el dashboard
    Then veo botones de acción rápida:
      | acción                | descripción                |
      | Create Agent          | abre formulario de agente  |
      | Run Flow              | abre selector de flow      |
      | View Memories         | navega a memories          |
      | View Cost Analytics   | navega a cost analytics    |

  Scenario: Status cards
    When veo el dashboard
    Then veo el estado del sistema:
      | componente    | estado (healthy/warning/error) |
      | API Server    | ok / error                     |
      | Database      | ok / error                     |
      | LLM Providers | conectado / desconectado      |
    And cada status card tiene color: verde/amarillo/rojo

  Scenario: Dashboard refresca datos periódicamente
    When espero 30 segundos en el dashboard
    Then los datos se refrescan automáticamente
    And veo un indicador de "last updated: Xs ago"

  Scenario: Dashboard responsivo
    When veo el dashboard en mobile (320px width)
    Then las cards se apilan verticalmente
    And sigue siendo legible

  Scenario: Sin datos muestra empty state
    Given no hay agentes, flows, skills ni runs
    When veo el dashboard
    Then veo mensajes "No agents yet. Create your first agent."
    And los botones de acción rápida están destacados

  Scenario: Error al cargar datos
    Given la API no responde
    When veo el dashboard
    Then veo un mensaje de error "Unable to load dashboard data"
    And un botón "Retry"
```

## Análisis breve

- **Qué pide realmente:** Frontend web (SPA) con dashboard que consume los endpoints de la API. Stats, activity feed, cost summary, status checks. Auto-refresh. Responsive.
- **Módulos sospechados:** `web/` (frontend app), `internal/api/handlers/dashboard.go`
- **Riesgos / dependencias:** Frontend framework a decidir (React/Vue/Svelte). Depende de REQ-13, REQ-15. Diseño UX.
- **Esfuerzo tentativo:** L

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
