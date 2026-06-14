# Design: issue-38.6-ci-build-backend

## Decisión arquitectónica

- **GitHub Actions** como plataforma CI (gratis para repos públicos).
- **GHCR** como registry (autenticación nativa con GITHUB_TOKEN).
- **Trigger por tag** (no por branch push) para release explícito.
- **Multi-arch (amd64 + arm64)** via QEMU + buildx.
- **Cache buildx** con backend GitHub Actions Cache (`gha`).
- **docker/build-push-action@v5** como step principal (oficial Docker).
- **docker/metadata-action@v5** para extraer tags + labels desde el git tag.

## Alternativas descartadas

- **GitLab CI**: no aplica, repo está en GitHub.
- **Docker Hub**: requiere cuenta + tokens manuales. GHCR es nativo.
- **Build solo amd64**: limita al user con Mac M-series. arm64 es free
  con QEMU emulation.
- **goreleaser para imágenes**: el .goreleaser.yml actual genera binarios
  pero no necesariamente push de imágenes. CI separado para Docker es
  más controlable y debugeable.
- **Push automático en cada commit a main**: confuso (qué versión es
  `latest`? la última?). Mejor explícito con tags.
- **Single-arch fallback (solo amd64)**: pierde devs ARM.

## Workflow YAML completo

```yaml
name: Build & push domain-backend image

on:
  push:
    tags:
      - 'backend-v*'
    paths:
      - 'domain-backend/**'

permissions:
  contents: read
  packages: write

jobs:
  build-and-push:
    runs-on: ubuntu-latest

    steps:
      - name: Checkout
        uses: actions/checkout@v4

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Login to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Extract metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ghcr.io/nunezlagos/domain-backend
          tags: |
            type=match,pattern=backend-v(.*),group=1
            type=raw,value=latest,enable=${{ !contains(github.ref, '-rc') && !contains(github.ref, '-beta') }}

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: domain-backend
          file: domain-backend/Dockerfile
          platforms: linux/amd64,linux/arm64
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
          build-args: |
            VERSION=${{ steps.meta.outputs.version }}
            COMMIT=${{ github.sha }}
            BUILD_TIME=${{ github.event.head_commit.timestamp }}
```

## Por qué `match` pattern para tags

`type=match,pattern=backend-v(.*),group=1` extrae lo que sigue a `backend-v`
y lo usa como tag. Ejemplos:
- Git tag `backend-v1.2.3` → image tag `v1.2.3`
- Git tag `backend-v2.0.0-rc1` → image tag `v2.0.0-rc1`
- Git tag `backend-v0.0.0-test` → image tag `v0.0.0-test`

## Por qué condicional para `latest`

`enable=${{ !contains(github.ref, '-rc') && !contains(github.ref, '-beta') }}`:
solo marca `latest` para versiones estables. RCs y betas no deberían
considerarse "latest" para deploys ad-hoc.

## Permisos

`packages: write` permite a `GITHUB_TOKEN` pushear a `ghcr.io/<owner>/...`.
`contents: read` para hacer checkout. No se necesita más.
