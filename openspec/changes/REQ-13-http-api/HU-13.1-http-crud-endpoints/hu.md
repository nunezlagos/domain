# HU-13.1-http-crud-endpoints

**Origen:** `REQ-13-http-api`
**Persona:** security-officer
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** desarrollador integrando con Domain
**Quiero** una API REST completa con endpoints CRUD para todas las entidades del sistema
**Para** gestionar organizaciones, usuarios, api_keys, proyectos, observaciones, sesiones, prompts, knowledge_docs, skills, agents, agent_runs, flows, flow_runs, crons, webhooks y audit_log desde clientes HTTP

## Criterios de aceptación

```gherkin
Feature: HTTP CRUD Endpoints

  Background:
    Given el servidor API corre en /api/v1/
    And todas las respuestas son JSON con Content-Type: application/json

  Scenario: Crear entidad via POST
    Given la entidad "observations" existe en el registro
    When envío POST /api/v1/observations con body JSON válido
    Then recibo 201 Created
    And el response body contiene la entidad creada con id único
    And el Location header apunta a /api/v1/observations/{id}

  Scenario: Obtener entidad por ID via GET
    Given existe una observación con id "abc-123"
    When envío GET /api/v1/observations/abc-123
    Then recibo 200 OK
    And el body contiene la observación completa

  Scenario: Listar entidades via GET
    Given existen 25 observaciones
    When envío GET /api/v1/observations
    Then recibo 200 OK
    And el body contiene un array en la clave "data"

  Scenario: Actualizar entidad via PUT (reemplazo completo)
    Given existe una observación con id "abc-123"
    When envío PUT /api/v1/observations/abc-123 con body JSON completo
    Then recibo 200 OK
    And todos los campos se reemplazan con los valores enviados

  Scenario: Actualizar parcialmente via PATCH
    Given existe una observación con id "abc-123"
    When envío PATCH /api/v1/observations/abc-123 con body JSON parcial
    Then recibo 200 OK
    And solo los campos enviados se modifican
    And el resto permanece igual

  Scenario: Eliminar entidad via DELETE
    Given existe una observación con id "abc-123"
    When envío DELETE /api/v1/observations/abc-123
    Then recibo 204 No Content
    And la entidad ya no existe en la base de datos

  Scenario: Obtener entidad inexistente retorna 404
    When envío GET /api/v1/observations/id-inexistente
    Then recibo 404 Not Found
    And el body contiene un error con código "not_found"

  Scenario: Crear entidad con body inválido retorna 422
    When envío POST /api/v1/observations con body JSON malformado
    Then recibo 422 Unprocessable Entity
    And el body contiene detalles de validación por campo

  Scenario: Endpoints consistentes para todas las entidades
    When consulto cualquier entidad via GET /api/v1/{entity}/{id}
    Then la estructura URL sigue el patrón /api/v1/{entity}/{id}
    And los códigos de respuesta son consistentes entre entidades
    And el formato de errores es idéntico entre entidades

  Scenario: Listar entidades con paginación por defecto
    When envío GET /api/v1/observations sin parámetros
    Then recibo los primeros 20 resultados por defecto
    And el response incluye metadatos de paginación

  Scenario: Crear organización root
    Given no existe ninguna organización
    When envío POST /api/v1/organizations con datos válidos
    Then recibo 201 Created
    And la organización se crea con rol "root"
```

## Análisis breve

- **Qué pide realmente:** Router REST genérico con handlers parametrizados por entidad. Validación de request body con schemas JSON. ORM queries estandarizadas. Consistent error responses.
- **Módulos sospechados:** `internal/api/`, `internal/handlers/`, `internal/router/`
- **Riesgos / dependencias:** Depende de schema de base de datos (REQ-01). La consistencia entre 16 entidades requiere abstracción genérica de handler. Volumen de endpoints (~80+ rutas).
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
