# issue-04.1-requirements-crud

**Origen:** `REQ-04-opsx-sdd`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** arquitecto de software gestionando un proyecto con SDD
**Quiero** crear, leer, actualizar, archivar y listar requisitos (REQs) con campos estructurados y jerarquía padre-hijo
**Para** mantener el backlog organizado y trazable desde el nivel más alto de requisitos

## Criterios de aceptación

```gherkin
Feature: Requirements CRUD

  Background:
    Given existe la tabla requirements en Postgres

  Scenario: Crear un requisito raíz
    When creo un requisito con:
      | slug        | "REQ-01-core-platform" |
      | title       | "Core Platform"        |
      | description | "Base de la plataforma" |
      | priority    | high                   |
    Then se persiste con status = "active"
    And parent_id es NULL
    And created_at y updated_at se setean

  Scenario: Crear un requisito hijo
    Given existe "REQ-01-core-platform" con id X
    When creo un requisito hijo:
      | slug        | "REQ-01.1-auth"    |
      | parent_slug | "REQ-01-core-platform" |
    Then su parent_id apunta a X
    And hereda ciertos atributos del padre (project scope)

  Scenario: Listar requisitos con filtros
    Given existen requisitos con diferentes status y prioridades
    When listo con status = "active"
    Then solo obtengo requisitos activos
    When listo con priority = "high"
    Then solo obtengo requisitos de alta prioridad

  Scenario: Obtener árbol jerárquico
    When consulto el árbol de "REQ-01-core-platform"
    Then obtengo el requisito raíz y todos sus hijos recursivamente
    And cada hijo incluye sus propios hijos

  Scenario: Archivar un requisito
    Given existe "REQ-01-core-platform" activo
    When archivo el requisito
    Then su status cambia a "archived"
    And sus hijos también se archivan (cascade opcional)
    And updated_at se actualiza

  Scenario: Actualizar un requisito
    When actualizo el título y descripción de un requisito
    Then los campos se modifican
    And updated_at se actualiza
    And el resto de campos permanece igual

  Scenario: Slug único
    When intento crear un requisito con slug existente
    Then recibo un error de UniqueViolation
```

## Análisis breve

- **Qué pide realmente:** Tabla `requirements` con slug único, jerarquía (parent_id), CRUD completo, filtros por status/priority, árbol jerárquico.
- **Módulos sospechados:** `internal/opsx/requirement.go`, `internal/store/pg/requirement.go`
- **Riesgos / dependencias:** Bajo. Tabla autocontenida con FK a sí misma.
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
