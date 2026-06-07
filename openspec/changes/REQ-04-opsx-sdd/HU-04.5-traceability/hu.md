# HU-04.5-traceability

**Origen:** `REQ-04-opsx-sdd`
**Persona:** dx-engineer, integrator
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario
**Como** arquitecto de software
**Quiero** consultar la trazabilidad completa desde REQ → HU → Spec → Design → Tasks → Code, y obtener reportes de cobertura, progreso y estado
**Para** tener visibilidad del estado del proyecto y poder tomar decisiones informadas

## Criterios de aceptación

```gherkin
Feature: Traceability Links and Reporting

  Background:
    Given existen REQs, HUs, proposals, designs, tasks y code references en el sistema

  Scenario: Trazabilidad completa de REQ a Code
    When consulto trazabilidad para "REQ-01-core-platform"
    Then obtengo:
      - El REQ con sus metadatos
      - Todas las HUs hijas con sus estados
      - Cada HU con su última proposal (si existe)
      - Cada HU con su último design (si existe)
      - Cada HU con sus tareas y progreso
      - Cada HU con referencias a código (si existen)

  Scenario: Trazabilidad inversa (Code → REQ)
    When consulto qué REQ están vinculados al archivo "internal/store/pg/observation.go"
    Then obtengo la cadena: archivo → HU → REQ

  Scenario: Dashboard de cobertura
    When consulto el dashboard de cobertura para el proyecto
    Then obtengo:
      - Total REQs: X (activos/archivados)
      - Total HUs: X (por status)
      - HUs con proposal: X (Y%)
      - HUs con design: X (Y%)
      - HUs con tareas completadas: X (Y%)
      - Cobertura general: X%

  Scenario: Reporte de progreso
    When consulto progreso por REQ
    Then obtengo para cada REQ:
      - HUs totales / completadas
      - Tareas totales / completadas
      - Progreso porcentual
    And ordenado por progreso ascendente (lo menos avanzado primero)

  Scenario: Cross-reference query
    When consulto todas las HUs sin proposal
    Then obtengo HUs que aún no tienen especificación
    When consulto todas las HUs sin design
    Then obtengo HUs sin diseño técnico
    When consulto todas las HUs con tareas incompletas
    Then obtengo HUs en progreso

  Scenario: Reporte de estado consolidado
    When consulto estado consolidado
    Then obtengo una matriz:
      | REQ                | HUs | Props | Designs | Tasks | Progress |
      | REQ-01-core        | 3/3 | 3/3   | 2/3     | 8/10  | 80%      |
      | REQ-02-auth        | 2/2 | 1/2   | 1/2     | 3/8   | 37%      |
```

## Análisis breve

- **Qué pide realmente:** Queries que cruzan REQ → HU → Proposal → Design → Task. Code references (tabla externa o columna). Dashboards y reportes agregados.
- **Módulos sospechados:** `internal/opsx/traceability.go`, `internal/store/pg/traceability.go`
- **Riesgos / dependencias:** Depende de HU-04.1 hasta HU-04.4. Code references requieren integración con el VCS o una tabla de mapeo archivo→HU.
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
