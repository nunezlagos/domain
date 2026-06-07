# HU-07.3-sync-status

**Origen:** `REQ-07-git-sync`
**Prioridad:** media
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria que sincroniza entre máquinas
**Quiero** ver el estado de sincronización: cuántos registros hay localmente, cuántos hay en los chunks del manifest, y si el manifest está íntegro
**Para** saber si necesito exportar o importar, y detectar problemas antes de sincronizar

## Criterios de aceptación

```gherkin
Scenario: engram sync --status muestra conteo local
  Given hay observaciones y sesiones en la base de datos local
  When el usuario ejecuta "engram sync --status"
  Then se muestra "Local: 1,234 observations, 56 sessions"
  And los conteos son precisos (SELECT COUNT de cada tabla)

Scenario: --status muestra conteo remoto desde manifest
  Given existe .engram/manifest.json con chunks
  When el usuario ejecuta "engram sync --status"
  Then se muestra "Remote: 567 observations, 23 sessions (in 5 chunks)"

Scenario: --status muestra diferencia local vs remoto
  Given local tiene 1000 observaciones y remote tiene 800
  When el usuario ejecuta "engram sync --status"
  Then se muestra "Diff: +200 observations local, +0 sessions"
  And se sugiere "Run 'engram sync' to export local changes"

Scenario: --status muestra manifest health check
  Given .engram/manifest.json existe
  When el usuario ejecuta "engram sync --status"
  Then se verifica: todos los chunks existen en disco, SHA-256 coincide
  And se muestra "Manifest: healthy (5/5 chunks verified)" o "Manifest: 1/5 chunks missing"

Scenario: --status sin manifest muestra advertencia
  Given no existe .engram/manifest.json
  When el usuario ejecuta "engram sync --status"
  Then se muestra "No manifest found. Run 'engram sync' to create one."
  And se muestran solo los conteos locales

Scenario: --status muestra timestamp del último sync
  Given existe .engram/manifest.json
  When el usuario ejecuta "engram sync --status"
  Then se muestra "Last export: 2026-06-01T14:30:00Z"

Scenario: Health check detecta chunk faltante
  Given manifest.json referencia un chunk que no existe en disco
  When el usuario ejecuta "engram sync --status"
  Then se muestra "Chunk a1b2c3d4: MISSING"
  And el health check general es "degraded"

Scenario: Health check detecta SHA-256 mismatch
  Given un chunk en disco tiene contenido diferente al esperado
  When el usuario ejecuta "engram sync --status"
  Then se muestra "Chunk a1b2c3d4: SHA-256 mismatch"
  And el health check general es "corrupt"
```

## Análisis breve

- **Qué pide realmente:** Modo status que muestra conteos locales (store), remotos (manifest), diff, health check de manifest (existencia + SHA-256 de cada chunk).
- **Módulos sospechados:** `internal/sync/status.go` — `StatusReport`, `HealthCheck`, `RemoteCounts`
- **Riesgos / dependencias:** Depende de store (counts) y manifest (remote counts). Health check requiere leer todos los chunks (I/O).
- **Esfuerzo tentativo:** S

## Verificación previa

- [x] Revisar codebase (grep) — greenfield
- [x] Revisar schema — sync_chunks existe
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** greenfield
- **Evidencia:** Sin Go code en el proyecto
- **Acción derivada:** Crear status.go con StatusReport y HealthCheck
