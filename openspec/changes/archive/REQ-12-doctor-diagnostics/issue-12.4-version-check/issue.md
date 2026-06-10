# issue-12.4-version-check

**Origen:** `REQ-12-doctor-diagnostics`
**Prioridad:** baja
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria
**Quiero** que `engram version` me muestre si hay una versión más nueva disponible
**Para** saber cuándo actualizar y no perder fixes o features nuevas

**Como** operador
**Quiero** que el version check tenga frecuencia configurable para no rate-limitear GitHub APIs
**Para** no ser bloqueado por exceder límites de requests

## Criterios de aceptación

```gherkin
Scenario: Up-to-date muestra mensaje de versión actual
  Given la versión instalada es v1.2.3
  And GitHub releases tiene v1.2.3 como latest
  When se ejecuta `engram version`
  Then output incluye "memoria v1.2.3"
  And output incluye "✓ up-to-date"

Scenario: Update available muestra nuevo tag
  Given la versión instalada es v1.2.3
  And GitHub releases tiene v2.0.0 como latest
  When se ejecuta `engram version`
  Then output incluye "memoria v1.2.3"
  And output incluye "⚠ update available: v2.0.0"
  And output incluye comando de actualización

Scenario: Check failed (no internet) no bloquea
  Given no hay conexión a internet
  When se ejecuta `engram version`
  Then output incluye "memoria v1.2.3"
  And output incluye "⚠ version check failed (offline)"
  And exit code es 0 (no error)

Scenario: Version format display completo
  Given memoria v1.2.3 (commit abc1234, build 2026-06-07)
  When se ejecuta `engram version`
  Then output incluye:
  - "memoria v1.2.3"
  - commit hash (abreviado)
  - build date
  - go version + OS/arch

Scenario: CheckFrequency evita rate limiting
  Given CheckFrequency = 1h
  When se hace version check dos veces en 1 minuto
  Then la segunda vez usa resultado en caché
  And no hace request HTTP a GitHub

Scenario: Cache expira después de CheckFrequency
  Given CheckFrequency = 1h
  When pasan más de 60 minutos desde el último check
  Then el próximo version check hace request HTTP

Scenario: GitHub API error graceful
  Given GitHub API retorna 403 rate limited
  When se ejecuta `engram version`
  Then output incluye "⚠ version check failed (rate limited)"
  And no crash

Scenario: Pre-release no se considera latest
  Given latest release es v2.0.0-rc.1
  And stable release es v1.9.0
  When se ejecuta version check
  Then latest considerado es v1.9.0 (no pre-release)

Scenario: `engram version --check` fuerza check online
  Given hay caché de version check
  When se ejecuta `engram version --check`
  Then fuerza request HTTP a GitHub
  And actualiza caché

Scenario: `engram version --json` incluye update info
  Given update available
  When se ejecuta `engram version --json`
  Then output JSON incluye current_version, latest_version, update_available
```

## Análisis breve

- **Qué pide realmente:** Version check contra GitHub Releases API con caché y frecuencia configurable, integración en `engram version`, manejo graceful de errores de red
- **Módulos sospechados:** `internal/version/` — agregar version_check.go, cache.go; `internal/cli/version.go` — modificar para incluir update check
- **Riesgos / dependencias:** GitHub API rate limiting (60 req/h sin auth); usar caché con frecuencia mínima de 1 hora; pre-releases no deben considerarse latest
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
- **Evidencia:** —
- **Acción derivada:** —
