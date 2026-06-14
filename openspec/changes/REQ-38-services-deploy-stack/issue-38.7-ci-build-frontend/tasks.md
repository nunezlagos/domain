# Tasks: issue-38.7-ci-build-frontend

## Workflow file

- [ ] **wf-001**: Crear `.github/workflows/build-frontend.yml`.
- [ ] **wf-002**: Trigger `on: push: tags: ['frontend-v*']` + `paths:
      ['domain-frontend/**']`.
- [ ] **wf-003**: Permissions `contents: read, packages: write`.
- [ ] **wf-004**: Steps idénticos a build-backend.yml pero con:
      - context: domain-frontend
      - file: domain-frontend/Dockerfile
      - images: ghcr.io/nunezlagos/domain-frontend
      - pattern: frontend-v(.*)

## Validación

- [ ] **test-001**: Tag `frontend-v0.0.0-test` + push → workflow se dispara.
- [ ] **test-002**: Workflow completa exit 0 en <5 min (placeholder simple).
- [ ] **test-003**: Imagen en GHCR: `ghcr.io/nunezlagos/domain-frontend:v0.0.0-test`.
- [ ] **test-004**: Manifest multi-arch correcto.
- [ ] **test-005**: `docker pull` exit 0 desde otra máquina.
- [ ] **test-006**: `docker run --rm -p 8080:80 ghcr.io/nunezlagos/domain-frontend:v0.0.0-test`
      sirve el placeholder.
- [ ] **test-007**: `curl http://localhost:8080/` devuelve "Coming soon".
- [ ] **test-008**: Path filter: cambio en `domain-backend/...` con tag
      `frontend-v...` NO dispara este workflow (path filter lo bloquea).
- [ ] **test-009**: Cleanup: borrar tag y package version de prueba.

## Edge cases

- Iguales a HU-38.6, aplicados al contexto frontend.

## Notas para reviewers

- Archivo NUEVO en `.github/workflows/build-frontend.yml`.
- Estructura espejo de build-backend.yml. Cambio mecánico de paths/tags.
- Cuando llegue Angular real, este workflow seguirá funcionando — el
  Dockerfile internamente cambiará para incluir un stage Node, pero el
  workflow lo build-pushea sin cambios.
