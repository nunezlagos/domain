# Design: issue-38.1-backend-dockerfile-refine

## Decisión arquitectónica

- **Base runtime:** `gcr.io/distroless/static-debian12:nonroot` (mantenido).
- **Base builder:** `golang:1.25-alpine` (mantenido).
- **Multi-stage:** 2 stages (builder + runtime). No agregar más.
- **Binarios incluidos:** solo `domain` + `domain-mcp`. Los otros 6 de cmd/
  (lints, schema-drift, anonymize) NO van en producción.
- **Label image canónico:** `ghcr.io/nunezlagos/domain-backend` (con sufijo).
- **User runtime:** `nonroot:nonroot` (distroless default).

## Alternativas descartadas

- **Cambiar a alpine como runtime:** distroless es más seguro (cero shell,
  cero package manager, menor superficie). Alpine tendría sentido si
  necesitáramos shell/wget para healthchecks complejos; el healthcheck del
  binary cubre el caso.
- **Imagen scratch:** ~5 MB más liviano, pero pierde CA certs y timezone
  data. Distroless static incluye eso por default.
- **Empacar todos los binarios de cmd/:** infla imagen sin aporte productivo
  (lints son tooling de dev, no de runtime).
- **Buildear con CGO:** innecesario, todo Go puro funciona sin CGO. Permite
  static binary trivial.

## Topología de stages

```
┌─ Stage 1: builder (golang:1.25-alpine) ──────────────────────────┐
│ WORKDIR /src                                                      │
│ COPY go.mod go.sum  → RUN go mod download   (cache layer)        │
│ COPY .              → RUN go build domain + domain-mcp           │
│ Output: /out/domain, /out/domain-mcp                              │
└──────────────────────────────────────────────────────────────────┘
                              │
                              ▼ (solo copia binarios)
┌─ Stage 2: runtime (distroless/static:nonroot) ───────────────────┐
│ COPY /out/domain     → /usr/local/bin/domain                     │
│ COPY /out/domain-mcp → /usr/local/bin/domain-mcp                 │
│ USER nonroot:nonroot                                              │
│ EXPOSE 8000                                                       │
│ HEALTHCHECK domain healthcheck                                    │
│ ENTRYPOINT ["/usr/local/bin/domain"]                              │
│ CMD ["server"]                                                    │
└──────────────────────────────────────────────────────────────────┘
```

## .dockerignore deltas vs actual

Sumar:
```
# Diagramas y assets de docs
*.png
*.svg
*.mmd
*.html

# Reports y outputs locales
reports/
*.log

# Tests y testdata
*_test.go
testdata/

# Compose de dev local (no va en runtime)
docker-compose.yml

# Repo metadata no necesario
README.md
CHANGELOG.md
ROADMAP.md
AGENTS.md
.github/
.goreleaser.yml
.squawk.toml
.stateless-allowed.yaml
.commitlintrc.json

# Specs y SDKs no van en imagen
openspec/
sdks/
```

Conservar (ya excluido):
- `.git`, `.github`, `.claude`, `.idea`, `.vscode`
- `*.md`, `docs`
- `*_test.go`, `testdata`
- `.env*`, `*.log`
- `node_modules`, `dist`, `bin`
