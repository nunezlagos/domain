# HU-05.6-http-stats-doctor

**Origen:** `REQ-05-http-api`
**Prioridad:** media
**Tipo:** feature

## Historia de usuario

**Como** administrador de memoria
**Quiero** endpoints HTTP para estadísticas, diagnóstico y health check
**Para** monitorear el estado de la base de datos y detectar problemas sin usar la CLI

## Criterios de aceptación

```gherkin
Feature: HTTP Stats, Doctor, and Health API
  As a memoria operator
  I want to query statistics, run diagnostics, and check health via REST
  So that I can monitor and troubleshoot the system programmatically

  Background:
    Given the memoria HTTP server is running on localhost:7437
    And there are sessions, observations, and prompts in the database

  Scenario: Get database statistics
    When I send a GET request to /stats
    Then the response status is 200
    And the response body contains:
      | key                 | type   |
      | total_observations  | int    |
      | total_sessions      | int    |
      | total_prompts       | int    |
      | total_projects      | int    |
      | db_size_bytes       | int    |
      | db_path             | string |
      | oldest_observation  | string |
      | newest_observation  | string |

  Scenario: Stats reflects session counts correctly
    Given there are 3 sessions in the database
    When I get /stats
    Then total_sessions is 3

  Scenario: Run doctor diagnostics
    When I send a GET request to /doctor
    Then the response status is 200
    And the response body is an array of checks
    And each check has:
      | name    | string |
      | status  | string |
      | message | string |

  Scenario: Doctor checks include orphans
    When I run /doctor
    Then one of the checks is "orphan_observations"
    And it reports observations without valid sessions

  Scenario: Doctor checks include FTS5 integrity
    When I run /doctor
    Then one of the checks is "fts5_index"
    And it verifies FTS5 sync is working

  Scenario: Doctor with specific check
    When I send a GET request to /doctor?check=orphans
    Then the response contains only the "orphan_observations" check

  Scenario: Doctor with project filter
    When I send a GET request to /doctor?project=myapp
    Then checks are scoped to project "myapp"

  Scenario: Health check returns OK
    When I send a GET request to /health
    Then the response status is 200
    And the response body contains:
      | status  | "ok"       |
      | version | "<semver>" |
      | uptime  | string     |

  Scenario: Health check is fast
    When I send a GET request to /health
    Then the response time is less than 100ms
```

## Análisis breve

- **Qué pide realmente:** 3 endpoints: (1) GET /stats con conteos de DB y metadata, (2) GET /doctor con checks de diagnóstico (orphans, FTS5, integrity, etc.) y filtro opcional por check/project, (3) GET /health con status rápido, version y uptime.
- **Módulos sospechados:** `internal/api/stats.go` (handlers), `internal/store/stats.go` (queries de agregación), `internal/doctor/` (diagnóstico, HU-12.1), `internal/version/` (build info)
- **Riesgos / dependencias:** Depende de HU-12.1 (doctor readonly checks) y HU-12.3 (health/version). Stats queries deben ser eficientes.
- **Esfuerzo tentativo:** S

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
