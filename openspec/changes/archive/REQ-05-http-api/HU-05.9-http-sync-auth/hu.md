# HU-05.9-http-sync-auth

**Origen:** `REQ-05-http-api`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** administrador de memoria
**Quiero** un endpoint para consultar el estado de sincronización con reason codes y stage de upgrade, y autenticación Bearer token para rutas sensibles
**Para** monitorear la sincronización y proteger operaciones destructivas desde la API REST

## Criterios de aceptación

```gherkin
Feature: HTTP Sync Status and Bearer Auth
  As a memoria operator
  I want sync status with reason codes and Bearer auth for sensitive routes
  So that I can monitor sync health and protect destructive operations

  Background:
    Given the memoria HTTP server is running on localhost:7437
    And ENGRAM_HTTP_TOKEN is set to "test-token-123"

  Scenario: Get sync status returns all fields
    When I send a GET request to /sync/status
    Then the response status is 200
    And the response body contains:
      | key            | type   |
      | sync_state     | string |
      | reason         | string |
      | reason_code    | int    |
      | upgrade_stage  | string |
      | last_sync_at   | string |
      | pending_chunks | int    |

  Scenario: Sync state is idle when no sync in progress
    When no sync is active
    And I send a GET request to /sync/status
    Then sync_state is "idle"
    And reason_code is 0

  Scenario: Sync state reflects active sync
    When a git sync operation is in progress
    And I send a GET request to /sync/status
    Then sync_state is "syncing"
    And reason_code is 1

  Scenario: Sync state with error after failed sync
    When the last sync failed due to network error
    Then sync_state is "error"
    And reason_code is 2
    And reason contains "network"

  Scenario: DELETE /sessions/{id} requires auth
    When I send a DELETE request to /sessions/s1 without Authorization header
    Then the response status is 401

  Scenario: DELETE /sessions/{id} with valid token succeeds
    When I send a DELETE request to /sessions/s1 with Authorization: Bearer test-token-123
    Then the response status is 204

  Scenario: DELETE /sessions/{id} with invalid token returns 401
    When I send a DELETE request to /sessions/s1 with Authorization: Bearer wrong-token
    Then the response status is 401

  Scenario: GET /export requires auth
    When I send a GET request to /export?project=test without token
    Then the response status is 401

  Scenario: POST /import requires auth
    When I send a POST request to /import without token
    Then the response status is 401

  Scenario: POST /projects/migrate requires auth
    When I send a POST request to /projects/migrate without token
    Then the response status is 401

  Scenario: GET /health does NOT require auth
    When I send a GET request to /health without token
    Then the response status is 200

  Scenario: GET /stats does NOT require auth
    When I send a GET request to /stats without token
    Then the response status is 200

  Scenario: GET /sync/status does NOT require auth
    When I send a GET request to /sync/status without token
    Then the response status is 200

  Scenario: Server starts without ENGRAM_HTTP_TOKEN
    When ENGRAM_HTTP_TOKEN is not set
    And I start the server
    Then protected routes return 500 with "token not configured"
    And non-protected routes work normally

  Scenario: Upgrade stage transitions
    When no upgrade has been performed
    Then upgrade_stage is "none"
    When a migration is in progress
    Then upgrade_stage is "migrating"
    When migration completes
    Then upgrade_stage is "complete"
```

## Análisis breve

- **Qué pide realmente:** (1) GET /sync/status con estado de sincronización (idle/syncing/error), reason codes numéricos, upgrade stage, metadata. (2) Sistema de autenticación Bearer vía ENGRAM_HTTP_TOKEN que protege DELETE en sessions/observations, GET /export, POST /import, POST /projects/migrate.
- **Módulos sospechados:** `internal/api/sync.go` (sync status handler), `internal/api/middleware.go` (auth middleware), `internal/sync/status.go` (sync state tracking, HU-07.3), `cmd/engram/` (env var loading)
- **Riesgos / dependencias:** Depende de HU-07.3 (sync status) para los datos de sync. El auth middleware debe aplicarse a rutas específicas de otras HUs.
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
