# Proposal: issue-38.7-ci-build-frontend

## Intención

Crear `.github/workflows/build-frontend.yml` análogo al backend pero para
`domain-frontend/`, disparado por tags `frontend-v*`, publicando a
`ghcr.io/nunezlagos/domain-frontend:vX.Y.Z`.

## Scope

**Incluye:**
- Workflow estructuralmente idéntico al `build-backend.yml` (HU-38.6) con:
  - Trigger: `push: tags: ['frontend-v*']`
  - Path filter: `domain-frontend/**`
  - context: `domain-frontend`
  - file: `domain-frontend/Dockerfile`
  - images: `ghcr.io/nunezlagos/domain-frontend`
  - Multi-arch: linux/amd64, linux/arm64
  - Build-args: VERSION, COMMIT, BUILD_TIME

**No incluye:**
- Tests de Angular (cuando exista código real, otro workflow puede correr
  `ng test`).
- Lighthouse / bundle size checks — futuro opcional.

## Enfoque técnico

Copy + paste del build-backend.yml con:
- `tags: ['frontend-v*']` en lugar de `backend-v*`
- `pattern=frontend-v(.*)` en metadata
- `context: domain-frontend` y `file: domain-frontend/Dockerfile`
- `images: ghcr.io/nunezlagos/domain-frontend`

## Riesgos

- **Mismos que HU-38.6**: cache size, emulación ARM, permisos GHCR.
- **Build placeholder es trivial**: nginx + index.html → build extremadamente
  rápido (<2 min). No hay riesgo de timeout.

## Testing

- Análogo a HU-38.6 pero para frontend:
  - Tag `frontend-v0.0.0-test`, push, esperar workflow.
  - `docker pull ghcr.io/nunezlagos/domain-frontend:v0.0.0-test`.
  - `docker run --rm -p 8080:80 ghcr.io/nunezlagos/domain-frontend:v0.0.0-test`.
  - `curl http://localhost:8080/` devuelve placeholder HTML.
- Delete tag post-validation.
