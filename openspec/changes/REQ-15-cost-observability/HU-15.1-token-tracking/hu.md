# HU-15.1-token-tracking

**Origen:** `REQ-15-cost-observability`
**Persona:** org-owner, org-admin
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** administrador de la Plataforma Domain
**Quiero** trackear automáticamente el uso de tokens por cada llamada LLM
**Para** entender el consumo, calcular costos y optimizar el uso de modelos

## Criterios de aceptación

```gherkin
Feature: Token Tracking

  Background:
    Given el sistema tiene integración con proveedores LLM (OpenAI, Anthropic, Ollama, etc.)
    And existe un modelo registrado con sus costos por token

  Scenario: Tracking automático en cada llamada LLM
    When se ejecuta una llamada LLM (via agent, flow, skill)
    Then se registra automáticamente un token usage record con:
      | campo         | descripción                        |
      | run_id        | ID de la ejecución                 |
      | model         | "gpt-4", "claude-3-opus", etc.     |
      | provider      | "openai", "anthropic", etc.        |
      | input_tokens  | cantidad de tokens de input        |
      | output_tokens | cantidad de tokens de output       |
      | total_tokens  | input + output                     |
      | cost          | costo calculado según modelo       |
      | timestamp     | ISO8601                            |

  Scenario: Cálculo de costo automático
    Given el modelo "gpt-4" tiene costo $0.03/1K input y $0.06/1K output
    When se registran 1000 input_tokens y 500 output_tokens
    Then el costo calculado es $0.03 * 1 + $0.06 * 0.5 = $0.06

  Scenario: Tracking por agente
    When un agente hace múltiples llamadas LLM
    Then todas las llamadas se asocian al mismo agent_run_id
    And puedo consultar el total de tokens por domain_agent_run

  Scenario: Tracking por flow
    When un flow ejecuta varios pasos que llaman LLM
    Then cada paso tiene su propio token usage
    And todos asociados al mismo flow_run_id

  Scenario: Agregación por proyecto
    Given existen runs en múltiples proyectos
    When consulto token usage por proyecto
    Then obtengo total de tokens y costo agregado

  Scenario: Agregación por fecha
    When consulto token usage por rango de fechas
    Then obtengo total de tokens y costo en ese período

  Scenario: Agregación por modelo
    When consulto token usage agrupado por modelo
    Then obtengo desglose: modelo, tokens, costo

  Scenario: Consulta de detalle
    When consulto un token usage específico por ID
    Then veo todos los campos del registro

  Scenario: Sin llamadas LLM no registra nada
    When se ejecuta un flow sin pasos LLM
    Then no se crean registros de token usage

  Scenario: Modelo no registrado usa costo default
    Given existe un modelo no registrado en la tabla de costos
    When se usa en una llamada
    Then se registra con cost = 0
    And se marca como "cost_unknown: true"
```

## Análisis breve

- **Qué pide realmente:** Middleware/hook en el LLM Provider Factory que después de cada llamada LLM persiste un registro en tabla `token_usage`. Consultas de agregación por run, proyecto, modelo, fecha.
- **Módulos sospechados:** `internal/llm/`, `internal/store/token_usage.go`, `internal/cost/`
- **Riesgos / dependencias:** Depende de LLM runners (REQ-06), model registry (REQ-06.4). La DB puede crecer rápido con muchas llamadas.
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
