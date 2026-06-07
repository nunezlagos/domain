# HU-04.2-user-stories-gherkin

**Origen:** `REQ-04-opsx-sdd`
**Persona:** dx-engineer, integrator
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** arquitecto de software
**Quiero** gestionar historias de usuario (HUs) con campos estructurados y criterios de aceptación en Gherkin
**Para** tener trazabilidad desde requisitos hasta implementación con escenarios verificables

## Criterios de aceptación

```gherkin
Feature: User Stories with Gherkin Scenarios

  Background:
    Given existe la tabla user_stories en Postgres
    And existe "REQ-01-core-platform" en requirements

  Scenario: Crear HU con Gherkin scenarios
    When creo una HU con:
      | slug        | "HU-01.1-db-schema"       |
      | title       | "Database Schema Migrations" |
      | description | "..."                     |
      | req_slug    | "REQ-01-core-platform"    |
      | priority    | high                      |
    And agrego escenarios Gherkin:
      | feature | Scenario                           | Given         | When                | Then          |
      | Schema  | Crear tabla users                  | schema existe | ejecuto migración   | tabla creada  |
      | Schema  | Rollback migración                 | tabla existe  | ejecuto rollback    | tabla eliminada|
    Then la HU se persiste con status = "proposed"
    And los escenarios se almacenan como structured data
    And la HU queda vinculada a "REQ-01-core-platform"

  Scenario: Gherkin almacenado como structured data
    When consulto una HU
    Then los escenarios están disponibles como:
      - feature (string)
      - scenario (string)
      - given (string[])
      - when (string)
      - then (string[])
    And no como texto plano

  Scenario: Actualizar escenarios Gherkin
    Given una HU con 2 escenarios
    When agrego un tercer escenario
    Then la HU tiene 3 escenarios
    When elimino el escenario 2
    Then la HU tiene 2 escenarios

  Scenario: Filtrar HUs por status y REQ
    Given HUs en diferentes REQs y statuses
    When filtro por req_slug = "REQ-01-core-platform"
    Then solo obtengo HUs de ese REQ
    When filtro por status = "active"
    Then solo obtengo HUs activas

  Scenario: Slug único por REQ
    When intento crear "HU-01.1-db-schema" dentro del mismo REQ
    Then recibo error de UniqueViolation (slug único global)
```

## Análisis breve

- **Qué pide realmente:** Tabla `user_stories` + tabla `gherkin_scenarios` (1:N). Escenarios con campos estructurados feature/scenario/given/when/then. FK a requirements.
- **Módulos sospechados:** `internal/opsx/user_story.go`, `internal/store/pg/user_story.go`
- **Riesgos / dependencias:** Depende de HU-04.1 (requirements table). Gherkin como structured data es más complejo que texto plano pero más útil para trazabilidad.
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
