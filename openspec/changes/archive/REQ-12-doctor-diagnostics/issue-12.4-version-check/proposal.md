# Proposal: issue-12.4-version-check

## Intención

Agregar version checking contra GitHub Releases API al comando `engram version`, con caché configurable para evitar rate limiting, y manejo graceful de errores de red.

## Scope

**Incluye:**
- `VersionChecker` que consulta GitHub Releases API
- Parsea latest release, ignorando pre-releases
- Caché con CheckFrequency (default 1h)
- `engram version` muestra update status
- `engram version --check` fuerza check online
- `--json` incluye update info
- Manejo de errores: offline, rate limited, API error

**No incluye:**
- Auto-update (solo notificación)
- Check en segundo plano (solo bajo demanda)
- Múltiples fuentes de release (solo GitHub)

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| API | GET https://api.github.com/repos/nunezlagos/memoria/releases/latest |
| Cache | In-memory map con timestamp; expira según CheckFrequency |
| CheckFrequency | Default 1h; configurable via `version.CheckFrequency` |
| Pre-releases | Filter: `!release.Prerelease` |
| JSON output | `VersionInfo` extendido con `LatestVersion`, `UpdateAvailable` |
