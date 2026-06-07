# HU-03.5-context-timeline

**Origen:** `REQ-03-memory-system`
**Persona:** dx-engineer, org-member
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario
**Como** agente de IA
**Quiero** obtener un resumen de contexto (últimas sesiones + observaciones + prompts) y una vista cronológica alrededor de una observación específica
**Para** tener awareness del estado actual del sistema y poder navegar el historial temporal

## Criterios de aceptación

```gherkin
Feature: Context and Timeline Queries

  Background:
    Given existen sesiones, observaciones y prompts en la base de datos

  Scenario: Obtener contexto completo del proyecto
    When consulto contexto para el proyecto "opencode-core"
    Then obtengo:
      | sección       | contenido                                            |
      | active_session| la sesión activa actual (si existe)                   |
      | recent_sessions| últimas 5 sesiones completadas                        |
      | recent_observations| últimas 10 observaciones                             |
      | recent_prompts | últimos 5 prompts                                     |
    And todos ordenados por created_at DESC

  Scenario: Contexto filtrado por scope
    When consulto contexto con scope = "project"
    Then solo obtengo entradas del proyecto actual
    When consulto contexto con scope = "personal"
    Then solo obtengo entradas personales
    When consulto contexto con scope = "global"
    Then solo obtengo entradas globales

  Scenario: Timeline cronológico alrededor de una observación
    Given existe una observación con id X creada en 2026-06-07T10:00:00Z
    When consulto timeline para la observación X
    Then obtengo 3 entradas anteriores y 3 posteriores
    And la observación X está en el medio
    And las entradas incluyen observaciones y prompts
    And el resultado está formateado para consumo del agente

  Scenario: Timeline sin suficientes entradas previas
    Given la observación X es la más antigua
    When consulto timeline para X
    Then obtengo 0 anteriores y hasta 6 posteriores

  Scenario: Formato para consumo de agente
    When consulto contexto
    Then el resultado está formateado como texto estructurado con secciones claras
    And incluye marcas de tiempo ISO 8601
    And incluye el tipo de cada entrada (session | observation | prompt)
```

## Análisis breve

- **Qué pide realmente:** Queries compuestas que unifican sessions, observations y prompts en una respuesta de contexto. Timeline muestra el "vecindario cronológico" de una entrada específica.
- **Módulos sospechados:** `internal/memory/context.go`, `internal/memory/timeline.go`
- **Riesgos / dependencias:** Depende de HU-03.1, HU-03.2, HU-03.3. Rendimiento con muchas entradas (limitar siempre).
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
