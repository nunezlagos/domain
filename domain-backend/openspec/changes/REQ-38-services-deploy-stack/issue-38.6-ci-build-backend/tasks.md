# Tasks: issue-38.6-ci-build-backend

## Workflow file

- [ ] **wf-001**: Crear `.github/workflows/build-backend.yml`.
- [ ] **wf-002**: Trigger `on: push: tags: ['backend-v*']` + `paths:
      ['domain-backend/**']`.
- [ ] **wf-003**: Permissions `contents: read, packages: write`.
- [ ] **wf-004**: Runner `ubuntu-latest`.
- [ ] **wf-005**: Step actions/checkout@v4.
- [ ] **wf-006**: Step docker/setup-qemu-action@v3.
- [ ] **wf-007**: Step docker/setup-buildx-action@v3.
- [ ] **wf-008**: Step docker/login-action@v3 a ghcr.io con GITHUB_TOKEN.
- [ ] **wf-009**: Step docker/metadata-action@v5 con pattern `backend-v(.*)`.
- [ ] **wf-010**: Step docker/build-push-action@v5:
      - context: domain-backend
      - file: domain-backend/Dockerfile
      - platforms: linux/amd64,linux/arm64
      - cache-from/to: type=gha (mode=max)
      - build-args: VERSION, COMMIT, BUILD_TIME.

## Validación

- [ ] **test-001**: Tag `backend-v0.0.0-test` + push → workflow se dispara.
- [ ] **test-002**: Workflow completa exit 0 en <15 min.
- [ ] **test-003**: Imagen aparece en GHCR:
      `ghcr.io/nunezlagos/domain-backend:v0.0.0-test`.
- [ ] **test-004**: Manifest multi-arch:
      `docker manifest inspect ghcr.io/nunezlagos/domain-backend:v0.0.0-test`
      muestra `linux/amd64` y `linux/arm64`.
- [ ] **test-005**: `docker pull ghcr.io/nunezlagos/domain-backend:v0.0.0-test`
      exit 0 desde cualquier máquina (visibilidad pública).
- [ ] **test-006**: `docker run --rm ghcr.io/nunezlagos/domain-backend:v0.0.0-test --version`
      imprime "v0.0.0-test" (build-arg inyectado).
- [ ] **test-007**: Build subsecuente (con cache) <5 min.
- [ ] **test-008**: Path filter funciona: push a `caddy/...` con tag
      `caddy-v1` (otro pattern) NO dispara este workflow.
- [ ] **test-009**: Cleanup: `git tag -d backend-v0.0.0-test && git push --delete origin backend-v0.0.0-test`.
- [ ] **test-010**: Borrar package version de prueba desde GitHub UI.

## Edge cases

- [ ] **edge-001**: Tag con formato inválido (`backend-vfoo`): metadata-action
      lo procesa pero el tag image queda raro. Aceptable; el operador no debe
      tagear así. Documentar convención SemVer en README.
- [ ] **edge-002**: Push de commit a main sin tag: workflow NO se dispara.
- [ ] **edge-003**: Tag pre-release (`backend-v1.0.0-rc1`): se publica como
      `v1.0.0-rc1` pero NO se etiqueta como `latest`.
- [ ] **edge-004**: Si Dockerfile tiene error de sintaxis, build falla en
      <2 min con mensaje claro. No se publica imagen.
- [ ] **edge-005**: Si GHCR está caído, workflow falla en step de push.
      Reintentar manualmente desde Actions UI.

## Notas para reviewers

- Archivo NUEVO en `.github/workflows/build-backend.yml`.
- NO modificar otros workflows (los que vinieron de main siguen igual).
- Tagging convention: `backend-vX.Y.Z` (SemVer). README documenta.
- Para test de validación: crear tag, esperar workflow, validar imagen,
  borrar tag de prueba. Documentar en task list.
