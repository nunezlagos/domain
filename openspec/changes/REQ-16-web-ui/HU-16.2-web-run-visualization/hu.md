# HU-16.2-web-run-visualization

**Origen:** `REQ-16-web-ui`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** desarrollador usando la Plataforma Domain
**Quiero** visualizar en tiempo real la ejecución de agentes y flows en el navegador
**Para** monitorear el progreso, identificar cuellos de botella y depurar errores paso a paso

## Criterios de aceptación

```gherkin
Feature: Web Run Visualization

  Background:
    Given existe un flow con múltiples pasos
    And el flow se está ejecutando

  Scenario: Flow run DAG viewer
    When navego a /runs/{run_id}
    Then veo el DAG del flow con todos los pasos
    And el paso actual está destacado con animación (pulsing)
    And los pasos completados tienen checkmark verde
    And los pasos fallidos tienen X roja
    And los pasos pendientes están en gris

  Scenario: DAG se actualiza en tiempo real
    When un paso se completa durante la visualización
    Then el DAG se actualiza sin recargar la página
    And el siguiente paso comienza a pulsar

  Scenario: Agent run log viewer (streaming)
    When veo un agent run en /runs/{run_id}
    Then veo los logs en tiempo real (streaming via SSE)
    And los logs tienen timestamp y nivel (info, warn, error)
    And los errores están en rojo
    And puedo hacer scroll para ver logs pasados

  Scenario: Step timing metrics
    When veo un run completado
    Then cada paso muestra:
      | métrica          | descripción                     |
      | duration         | tiempo que tomó el paso         |
      | input_tokens     | tokens de input usados          |
      | output_tokens    | tokens de output generados      |
      | cost             | costo del paso                  |
    And el tiempo total del run se muestra al inicio

  Scenario: Drill-down en cada paso
    When hago click en un paso del DAG
    Then veo el detalle del paso:
      | campo            | valor                           |
      | input            | prompt/data enviado             |
      | output           | respuesta del LLM               |
      | model            | modelo usado                    |
      | duration         | duración                        |
      | tokens           | conteo de tokens                |
      | error            | mensaje de error si falló       |

  Scenario: Run fallido muestra error
    Given un run falló en el paso 3
    When veo el run
    Then el DAG muestra paso 3 en rojo
    And el panel de detalle muestra el error message
    And un botón "Retry from this step"

  Scenario: Timeline view
    When veo un run completado
    Then tengo opción de ver timeline en lugar de DAG
    And el timeline muestra pasos en secuencia con barras de duración
    And puedo ver overlapp de pasos paralelos

  Scenario: Success/failure indicators
    When veo la lista de runs en /runs
    Then cada run tiene indicador: ✅ success, ❌ failed, ⏳ running
    And el color de fondo varía: verde/rojo/azul

  Scenario: Run sin pasos
    Given un run sin pasos registrados
    When veo el run
    Then veo mensaje "No steps recorded for this run"
```

## Análisis breve

- **Qué pide realmente:** Frontend de visualización de runs con DAG interactivo, logs streaming (SSE), métricas de paso, timeline view. Backend: SSE endpoint para streaming de logs (HU-11.3).
- **Módulos sospechados:** `web/` (frontend), `internal/api/handlers/runs.go` (SSE endpoint)
- **Riesgos / dependencias:** Depende de HU-11.3 (execution streaming), HU-09 (flow system), HU-08 (agent system). DAG rendering library.
- **Esfuerzo tentativo:** XL

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
