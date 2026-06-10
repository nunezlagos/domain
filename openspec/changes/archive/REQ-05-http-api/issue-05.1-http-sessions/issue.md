# issue-05.1-http-sessions

**Origen:** `REQ-05-http-api`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** desarrollador integrando memoria en herramientas externas
**Quiero** endpoints HTTP para crear, finalizar, listar y eliminar sesiones
**Para** gestionar el ciclo de vida de sesiones de memoria desde cualquier cliente HTTP

## Criterios de aceptación

```gherkin
Feature: HTTP Sessions API
  As an HTTP client
  I want to manage sessions via REST endpoints
  So that I can integrate memoria session lifecycle into external tools

  Background:
    Given the memoria HTTP server is running on localhost:7437
    And the Content-Type is application/json

  Scenario: Create a new session
    When I send a POST request to /sessions with:
      | project   | "myapp" |
      | directory | "/home/user/myapp" |
    Then the response status is 201
    And the response body contains "id"
    And the response body contains "project" with value "myapp"
    And the response body contains "directory" with value "/home/user/myapp"
    And the response body contains "started_at"
    And the response body contains "status" with value "active"

  Scenario: Create session with only required fields
    When I send a POST request to /sessions with:
      | directory | "/tmp" |
    Then the response status is 201
    And the response body contains "id"
    And the response body contains "project" with value "default"

  Scenario: End an active session
    Given an active session with id "s1" exists
    When I send a POST request to /sessions/s1/end
    Then the response status is 200
    And the response body contains "id" with value "s1"
    And the response body contains "ended_at"
    And the response body contains "status" with value "ended"

  Scenario: End an already ended session returns 409
    Given session "s1" is already ended
    When I send a POST request to /sessions/s1/end
    Then the response status is 409
    And the response body contains "error"

  Scenario: End a non-existent session returns 404
    When I send a POST request to /sessions/nonexistent/end
    Then the response status is 404

  Scenario: Get recent sessions
    Given there are 5 sessions with different started_at values
    When I send a GET request to /sessions/recent
    Then the response status is 200
    And the response body is an array
    And the array has at most 20 items
    And sessions are ordered by started_at descending

  Scenario: Get recent sessions with explicit limit
    When I send a GET request to /sessions/recent?limit=3
    Then the response array has at most 3 items

  Scenario: Get session by ID
    Given a session with id "s1" exists
    When I send a GET request to /sessions/s1
    Then the response status is 200
    And the response body contains "id" with value "s1"

  Scenario: Get non-existent session returns 404
    When I send a GET request to /sessions/nonexistent
    Then the response status is 404

  Scenario: Delete a session without observations
    Given a session "s1" exists with no observations
    When I send a DELETE request to /sessions/s1
    Then the response status is 204

  Scenario: Delete a session with observations returns 409
    Given a session "s1" exists with associated observations
    When I send a DELETE request to /sessions/s1
    Then the response status is 409
    And the response body contains "error"
    And the error message mentions "observations"

  Scenario: Delete non-existent session returns 404
    When I send a DELETE request to /sessions/nonexistent
    Then the response status is 404

  Scenario: POST /sessions with validation error returns 400
    When I send a POST request to /sessions with empty body
    Then the response status is 400
    And the response body contains "error"
```

## Análisis breve

- **Qué pide realmente:** 5 endpoints REST para CRUD de sesiones sobre `internal/store`, con validación de integridad referencial (DELETE bloqueado si tiene observations), manejo de conflictos (end de sesión ya finalizada), formato JSON consistente
- **Módulos sospechados:** `internal/api/` (nuevo paquete HTTP handler), `internal/store/sessions.go` (repo de sesiones, puede existir de issue-02.1), `cmd/engram/` (entrypoint serve)
- **Riesgos / dependencias:** Depende de issue-02.1 (session start/end) para la capa de store; delega en la store layers existentes; requieres sesiones en DB
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
