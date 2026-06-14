# Tasks: issue-38.1-backend-dockerfile-refine

## Dockerfile

- [ ] **dock-001**: Actualizar label `org.opencontainers.image.description`
      con texto descriptivo del backend.
- [ ] **dock-002**: Confirmar `LABEL org.opencontainers.image.source` apunta a
      `https://github.com/nunezlagos/domain` (canonical repo URL).
- [ ] **dock-003**: Verificar que `ARG VERSION`, `ARG COMMIT`, `ARG BUILD_TIME`
      están definidos y se pasan al `-X main.X=...`.
- [ ] **dock-004**: Verificar `USER nonroot:nonroot` en runtime stage.
- [ ] **dock-005**: Confirmar `EXPOSE 8000` alineado con `DOMAIN_HTTP_PORT`
      default del binary.
- [ ] **dock-006**: Confirmar `HEALTHCHECK` en formato exec form
      (`CMD ["...", "..."]`), no shell form.

## .dockerignore

- [ ] **ign-001**: Sumar excludes de diagramas: `*.png`, `*.svg`, `*.mmd`, `*.html`.
- [ ] **ign-002**: Sumar excludes de reports/logs: `reports/`, `*.log`.
- [ ] **ign-003**: Sumar exclude del `docker-compose.yml` (es de dev local).
- [ ] **ign-004**: Sumar exclude de metadata: `README.md`, `CHANGELOG.md`,
      `ROADMAP.md`, `AGENTS.md`.
- [ ] **ign-005**: Sumar excludes de CI/config: `.github/`, `.goreleaser.yml`,
      `.squawk.toml`, `.stateless-allowed.yaml`, `.commitlintrc.json`.
- [ ] **ign-006**: Sumar exclude de `openspec/` y `sdks/` (no van en runtime).

## Verificación local

- [ ] **test-001**: `docker buildx build -t domain-backend:dev --load domain-backend/`
      exit 0 en <5 min.
- [ ] **test-002**: `docker images domain-backend:dev` size <30 MB.
- [ ] **test-003**: `docker run --rm domain-backend:dev healthcheck` exit 0.
- [ ] **test-004**: `docker run --rm domain-backend:dev --version` imprime
      version correcta (inyectada via ldflags).
- [ ] **test-005**: `docker inspect domain-backend:dev | jq '.[0].Config.User'`
      igual a `"nonroot:nonroot"` o `"65532:65532"`.
- [ ] **test-006**: `docker inspect domain-backend:dev | jq '.[0].Config.Labels'`
      contiene los labels OCI nuevos.
- [ ] **test-007**: `docker history domain-backend:dev` muestra solo 2 stages.
- [ ] **test-008**: Build context con `--progress=plain` muestra envío <20 MB.

## Verificación end-to-end (post HU-38.2 también listo)

- [ ] **e2e-001**: Con `domain-internal` network + PG arriba + .env válido:
      `docker run --rm --network domain_internal -e DOMAIN_DATABASE_URL=... domain-backend:dev server`
      arranca y `/healthz` responde 200.

## Notas para reviewers

- Cambios SOLO en `domain-backend/Dockerfile` y `domain-backend/.dockerignore`.
  Sin tocar nada más.
- Si build falla local, NO mergear — fix antes.
- Reviewer debe correr build local también para confirmar reproducibilidad.
