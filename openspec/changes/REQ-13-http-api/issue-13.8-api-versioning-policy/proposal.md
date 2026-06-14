# Proposal: issue-13.8-api-versioning-policy

## Intención

Definir y enforzar política de API versioning: URL versioning, deprecation 12 meses min, RFC 8594 headers, sunset enforcement, docs publicados, endpoint /api/version.

## Scope

**Incluye:**
- Documento policy en `docs/api/versioning.md`
- Middleware que agrega Deprecation/Sunset headers por config
- Endpoint GET /api/version
- Support matrix `docs/api/support-matrix.md` actualizable
- 410 enforcement post sunset
- Changelog template `docs/api/changelog.md`

**No incluye:**
- Header-based versioning (Accept: application/vnd.domain.v2+json) — descartado por menos visible
- Auto-translation entre versions (futuro si necesario)

## Enfoque técnico

1. Config table `api_versions` con status, sunset_at, link
2. Middleware lee config y agrega headers
3. Endpoint /version expone matrix
4. CI valida changelog se actualiza con cada PR de API

## Riesgos

- Maintenance burden 2 versions: cap a 2 active simultáneas
- Sunset enforcement breaking: comunicación amplia + email a API keys activas

## Testing

- Headers Deprecation/Sunset presentes en v deprecated
- 410 post sunset
- /api/version correct
- CI fail si changelog no updated
