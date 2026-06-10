# issue-10.3-conflict-cli-api

**Origen:** `REQ-10-conflict-detection`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria
**Quiero** comandos CLI para gestionar conflictos: list, show, stats, scan, deferred
**Para** poder revisar y administrar conflictos sin entrar al dashboard

**Como** desarrollador
**Quiero** endpoints HTTP /conflicts/* para integrar conflictos en la API
**Para** construir herramientas y automatizaciones sobre el sistema de conflictos

## Criterios de aceptación

```gherkin
Scenario: `engram conflicts list` lista conflictos con filtros
  Given hay conflictos en memory_relations
  When se ejecuta `engram conflicts list --status pending --limit 20`
  Then mustra tabla con: ID, source, target, relation, status, confidence, created_at

Scenario: `engram conflicts show <id>` mustra detalle completo
  Given un conflicto con ID 42
  When se ejecuta `engram conflicts show 42`
  Then mustra: source content, target content, relation, judgment_status, confidence, reason, evidence

Scenario: `engram conflicts stats` mustra estadísticas
  Given hay conflictos en varios estados
  When se ejecuta `engram conflicts stats`
  Then mustra: total, pending, judged, error, por tipo de relación

Scenario: `engram conflicts scan` ejecuta FindCandidates
  Given observaciones en la DB
  When se ejecuta `engram conflicts scan --apply --max-insert=100 --since=24h`
  Then ejecuta FindCandidates con esos opts
  And mustra el ScanReport

Scenario: `engram conflicts deferred` lista la deferred queue
  Given hay entries en sync_apply_deferred
  When se ejecuta `engram conflicts deferred`
  Then mustra tabla con: sync_id, entity, status, retry_count, last_error

Scenario: `engram conflicts deferred replay <id>` reintenta un deferred
  Given un deferred entry con status="error"
  When se ejecuta `engram conflicts deferred replay 42`
  Then reintenta aplicar el entry
  And mustra resultado: success o error

Scenario: GET /conflicts lista conflictos
  Given hay conflictos en la DB
  When GET /conflicts?status=pending&limit=10
  Then retorna JSON array de conflictos

Scenario: GET /conflicts/:id mustra detalle
  Given un conflicto con ID 42
  When GET /conflicts/42
  Then retorna JSON con detalle completo

Scenario: POST /conflicts/:id/judge ejecuta JudgeBySemantic
  Given un conflicto pending con ID 42
  When POST /conflicts/42/judge
  Then ejecuta JudgeBySemantic
  And retorna judgment result

Scenario: POST /conflicts/scan ejecuta scan léxico
  Given un request POST /conflicts/scan con body {dry_run: true, max_insert: 50}
  When se procesa
  Then ejecuta FindCandidates con esos opts
  And retorna ScanReport como JSON
```

## Análisis breve

- **Qué pide realmente:** CLI commands (list, show, stats, scan, deferred) + HTTP endpoints (/conflicts/*) + deferred sync queue con replay
- **Módulos sospechados:** `internal/cli/conflicts.go`, `internal/api/conflicts.go`, `internal/conflict/deferred.go`
- **Riesgos / dependencias:** Duplicación lógica entre CLI y HTTP API; mantener consistencia
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
- **Evidencia:** —
- **Acción derivada:** —
