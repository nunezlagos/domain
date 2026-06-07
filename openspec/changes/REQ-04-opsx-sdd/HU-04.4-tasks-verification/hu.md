# HU-04.4-tasks-verification

**Origen:** `REQ-04-opsx-sdd`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** desarrollador implementando una HU
**Quiero** gestionar tareas con seguimiento de estado, resultados de verificación, registros de sabotaje y resultados de tests
**Para** tener visibilidad del progreso de implementación y garantizar calidad mediante verificación sistemática

## Criterios de aceptación

```gherkin
Feature: Tasks and Verification Management

  Background:
    Given existe "HU-01.1-db-schema" en user_stories

  Scenario: Crear tareas para una HU
    When creo tareas para "HU-01.1-db-schema":
      | section  | description                         |
      | Backend  | "Crear migración inicial"            |
      | Backend  | "Implementar store"                  |
      | Tests    | "Test de integración con pgtest"     |
      | Tests    | "Sabotaje: dropear índice"           |
      | Cierre   | "Verificación manual"                |
    Then cada tarea se persiste con status = "pending"
    And las tareas mantienen su sección (Backend | Tests | Cierre)
    And están ordenadas por section y position

  Scenario: Actualizar estado de tarea
    Given existe una tarea "Crear migración inicial" en estado "pending"
    When cambio su estado a "in_progress"
    Then started_at se setea
    When cambio su estado a "completed"
    Then completed_at se setea
    And se registra quién la completó

  Scenario: Registrar resultado de verificación
    Given existe una tarea completada
    When registro verificación con:
      | result  | evidence           | notes             |
      | pass    | "Test suite green" | "Todos los tests" |
    Then el resultado se persiste vinculado a la tarea
    And la tarea queda verificada

  Scenario: Registrar sabotaje
    Given existe una tarea de tipo "sabotaje"
    When ejecuto el sabotaje
    Then registro:
      | action      | "Dropear índice GIN"          |
      | result      | "Search falla con error claro"|
      | restored    | true                          |
    And el registro de sabotaje queda vinculado a la tarea

  Scenario: Ver resultados de tests por tarea
    When consulto una tarea
    Then obtengo:
      - estado actual
      - resultado de verificación (si existe)
      - registros de sabotaje (si existen)
      - resultados de tests asociados

  Scenario: Progress de HU
    Given una HU con 10 tareas
    When 6 están completadas
    Then el progreso es 60%
    And puedo consultar el resumen: X de Y tareas completadas
```

## Análisis breve

- **Qué pide realmente:** 3 tablas: `tasks`, `verification_results`, `sabotage_records`. FK a user_stories. Status tracking (pending → in_progress → completed). Progress calculation.
- **Módulos sospechados:** `internal/opsx/task.go`, `internal/store/pg/task.go`
- **Riesgos / dependencias:** Depende de HU-04.2. Verification y sabotage son extensibles (polimórficos).
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
