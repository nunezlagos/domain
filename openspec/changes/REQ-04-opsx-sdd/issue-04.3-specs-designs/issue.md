# issue-04.3-specs-designs

**Origen:** `REQ-04-opsx-sdd`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** arquitecto de software
**Quiero** crear y gestionar especificaciones (proposals) y diseños técnicos por cada HU, con metadatos estructurados y markdown
**Para** tener trazabilidad desde la intención hasta la implementación, con decisiones arquitectónicas documentadas y alternativas evaluadas

## Criterios de aceptación

```gherkin
Feature: Specs and Designs Management

  Background:
    Given existe "issue-01.1-db-schema" en issues

  Scenario: Crear proposal para una HU
    When creo una proposal para "issue-01.1-db-schema" con:
      | field              | value                                    |
      | intention          | "Implementar migraciones versionadas"     |
      | scope              | "Solo PostgreSQL, no MySQL"              |
      | approach           | "Usar golang-migrate con SQL embebido"   |
      | risks              | "Breaking changes en schema existente"   |
    Then la proposal se persiste vinculada a la HU
    And status = "draft"
    And created_at se setea

  Scenario: Crear design a partir de una proposal
    Given existe una proposal para "issue-01.1-db-schema"
    When creo un design con:
      | field              | value                                    |
      | architecture_decisions | "Tabla única con generated column"    |
      | alternatives       | "SQLite FTS5, Elasticsearch"            |
      | data_flow          | "App → Store → SQL → PG"                |
      | tdd_plan           | "Red: test migración → Green: impl"     |
    Then el design se persiste vinculado a la HU
    And referencea la proposal como origen

  Scenario: Almacenar como markdown + metadata
    When consulto una proposal
    Then el contenido del campo approach está en markdown
    And los metadatos (created_at, status, hu_slug) son consultables como structured data
    When consulto un design
    Then architecture_decisions está en markdown
    And los metadatos son consultables

  Scenario: Versionado de specs
    When actualizo una proposal existente
    Then se crea una nueva versión
    And la versión anterior permanece accesible
    And el campo version se incrementa

  Scenario: Listar proposals/designs por HU
    Given múltiples proposals para diferentes HUs
    When listo proposals para "issue-01.1-db-schema"
    Then solo obtengo las de esa HU
    When listo designs
    Then obtengo designs con su HU asociada

  Scenario: Status workflow de proposal
    Given una proposal en status "draft"
    When cambio status a "approved"
    Then updated_at se actualiza
    When cambio status a "rejected"
    Then se persiste el motivo de rechazo
```

## Análisis breve

- **Qué pide realmente:** 2 tablas: `proposals` y `designs`. Campos markdown + metadata estructurada. FK a issues. Versionado simple (número incremental). Status workflow (draft → approved/rejected).
- **Módulos sospechados:** `internal/opsx/spec.go`, `internal/store/pg/spec.go`
- **Riesgos / dependencias:** Depende de issue-04.2 (issues). Versionado puede ser costoso en writes frecuentes.
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
