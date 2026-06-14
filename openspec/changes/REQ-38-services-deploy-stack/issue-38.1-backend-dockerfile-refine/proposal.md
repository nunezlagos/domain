# Proposal: issue-38.1-backend-dockerfile-refine

## Intención

Refinar el `Dockerfile` y `.dockerignore` heredados de `main` para producir una
imagen `ghcr.io/nunezlagos/domain-backend:vX.Y.Z` mínima, reproducible y
publicable, sin tocar la estructura multi-stage ni el runtime distroless.

## Scope

**Incluye:**
- Rename del label OCI `org.opencontainers.image.source` para reflejar
  `ghcr.io/nunezlagos/domain-backend` (con sufijo `-backend`).
- Actualizar `LABEL org.opencontainers.image.description` a algo más descriptivo
  (ej. "Domain backend — multi-tenant context API + MCP HTTP server").
- Verificar que `ARG VERSION`/`COMMIT`/`BUILD_TIME` se inyectan correctamente
  via ldflags y que `domain version` los reporta.
- Refinar `.dockerignore` para excluir lo nuevo que copiamos a domain-backend/:
  PNGs, SVGs, mermaid, reports/, docker-compose.yml local de dev.
- Verificar que la imagen final pesa <30 MB (target distroless).
- Asegurar que `EXPOSE 8000` está alineado con `DOMAIN_HTTP_PORT` default.
- Asegurar que `USER nonroot:nonroot` está activo (no root en runtime).
- Asegurar que `HEALTHCHECK` invoca `domain healthcheck` correctamente.

**No incluye:**
- Cambio del base image (mantenemos distroless).
- Agregar binarios adicionales a la imagen (solo `domain` y `domain-mcp`).
- Refactor de la lógica de build de Go (ldflags, CGO, etc.).
- Configuración CI/CD (eso es HU-38.6).
- Compose de deploy en VPS (eso es HU-38.2).

## Enfoque técnico

1. **Lectura del Dockerfile actual**: ya está bien estructurado en 2 stages.
   Solo necesita ajustes cosméticos de labels y un par de líneas para metadata.
2. **Refinement de .dockerignore**: el actual excluye `.git`, `.claude`,
   `docs`, etc. Falta excluir:
   - `*.png`, `*.svg`, `*.mmd`, `*.html` (diagramas)
   - `reports/`, `*_test.go`, `testdata/`
   - `docker-compose.yml` (es el de dev local, no va en la imagen)
   - `Makefile`, `README.md`, `CHANGELOG.md`, `ROADMAP.md`, `AGENTS.md`
   - `.github/`, `.goreleaser.yml`, `.squawk.toml`, `.stateless-allowed.yaml`
   - `openspec/` (specs no entran a runtime)
   - `sdks/` (SDKs cliente, no del backend)
3. **Test local**: `docker buildx build -t domain-backend:dev --load .` debe
   compilar OK y la imagen final debe pesar <30 MB.
4. **Verify runtime**:
   - `docker run --rm domain-backend:dev healthcheck` retorna OK
   - `docker run --rm domain-backend:dev --version` imprime version
   - `docker run --rm domain-backend:dev server` arranca el HTTP server en :8000

## Riesgos

- **Distroless no tiene shell**: si el healthcheck usa shell tricks, hay que
  usar form exec directo. Mitigación: el HEALTHCHECK actual ya usa exec form
  (`CMD ["/usr/local/bin/domain", "healthcheck"]`).
- **Cambio de label rompe deploys existentes**: si el VPS ya pulleó la imagen
  vieja como `ghcr.io/nunezlagos/domain`, requiere update del `.env` cuando
  hagamos rename. Mitigación: hoy NO hay deploys de la imagen vieja en
  producción, este es el momento correcto para el rename.
- **`.dockerignore` muy agresivo puede romper build**: si se excluye algo que
  Go necesita (ej. vendor/), build falla. Mitigación: test local antes de
  push.
- **Binarios extras pesan más**: actualmente compilamos `domain` + `domain-mcp`.
  Si se sumara `domain-admin` u otros, la imagen creció. Mitigación: explicito
  qué binarios van.

## Testing

- `docker buildx build -t domain-backend:dev --load domain-backend/` exit 0
- `docker images domain-backend:dev` muestra size <30 MB
- `docker run --rm domain-backend:dev healthcheck` exit 0
- `docker run --rm -p 8000:8000 domain-backend:dev server &` arranca
- `curl http://localhost:8000/healthz` devuelve 200
- `docker inspect domain-backend:dev | jq '.[0].Config.Labels'` contiene
  `"org.opencontainers.image.source": "https://github.com/nunezlagos/domain"`
- `docker history domain-backend:dev` muestra solo 2 stages + COPY layers
- `docker run --rm domain-backend:dev whoami` falla (no shell) o muestra
  nonroot si lo hubiera (distroless)
- Build context size: `docker buildx build --progress=plain` muestra
  contexto <20 MB enviado al daemon
