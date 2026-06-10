# issue-15.2-cost-analytics

**Origen:** `REQ-15-cost-observability`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario
**Como** administrador de la plataforma
**Quiero** un dashboard de analytics de costos con breakdowns por dimensión y exportación CSV
**Para** entender dónde se gasta el presupuesto y tomar decisiones de optimización

## Criterios de aceptación

```gherkin
Feature: Cost Analytics

  Background:
    Given existen registros de token_usage en los últimos 90 días
    And el modelo de costos está configurado

  Scenario: Vista de gasto diario
    When consulto "spend daily" para los últimos 7 días
    Then obtengo un array con {date, total_cost, total_tokens} por día
    And ordenado por fecha ascendente

  Scenario: Vista de gasto semanal
    When consulto "spend weekly" para el último mes
    Then obtengo agregación semanal (lunes a domingo)

  Scenario: Vista de gasto mensual
    When consulto "spend monthly" para el último año
    Then obtengo agregación por mes calendario

  Scenario: Breakdown por proyecto
    When consulto breakdown por proyecto
    Then obtengo: proyecto, total_tokens, total_cost, % del total
    And ordenado por costo descendente

  Scenario: Breakdown por agente
    When consulto breakdown por agente
    Then obtengo por agente: tokens, costo, llamadas, modelo más usado

  Scenario: Breakdown por flow
    When consulto breakdown por flow
    Then obtengo por flow: tokens, costo, duración promedio

  Scenario: Breakdown por modelo
    When consulto breakdown por modelo
    Then obtengo: modelo, proveedor, tokens, costo, % del total

  Scenario: Breakdown por proveedor
    When consulto breakdown por proveedor (OpenAI vs Anthropic vs Ollama)
    Then obtengo: proveedor, total_tokens, total_cost

  Scenario: Cost forecasting
    When consulto forecast para el próximo mes
    Then obtengo proyección basada en promedio de últimos 30 días
    And incluye intervalo de confianza

  Scenario: Budget tracking
    Given existe un budget de $100/mes para el proyecto X
    When el gasto del mes actual es $85
    Then el estado es "75% consumed" (85/100)
    And se muestra progress bar

  Scenario: Budget warning
    Given existe un budget de $100/mes
    When el gasto supera $80 (80%)
    Then se marca como "warning: approaching limit"

  Scenario: Export a CSV
    When exporto "spend daily" a CSV
    Then descargo un archivo CSV con headers y datos
    And es compatible con Excel/Google Sheets

  Scenario: Export por breakdown
    When exporto breakdown por modelo a CSV
    Then obtengo CSV con columnas: model, provider, tokens, cost, percentage
```

## Análisis breve

- **Qué pide realmente:** Queries de agregación temporal (día/semana/mes) y dimensional (proyecto/agente/flow/modelo/proveedor). Cost forecasting simple (promedio móvil). Budget tracking con umbrales. Export CSV.
- **Módulos sospechados:** `internal/cost/analytics.go`, `internal/api/handlers/cost.go`, `internal/cost/forecast.go`
- **Riesgos / dependencias:** Depende de issue-15.1 (datos de token_usage). Queries pesadas en DB si hay muchos registros.
- **Esfuerzo tentativo:** M

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
