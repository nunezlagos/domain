# Design: issue-38.7-ci-build-frontend

## Decisión arquitectónica

Igual que HU-38.6 pero para `domain-frontend/`:
- GitHub Actions + GHCR.
- Trigger por tag `frontend-v*`.
- Multi-arch amd64+arm64.
- Cache buildx GitHub Actions.
- Sin tests automáticos (placeholder es estático; cuando llegue Angular,
  otro workflow separado puede correr `ng test`).

## Alternativas descartadas

- **Reusable workflow compartido entre backend y frontend**: introduce
  complejidad innecesaria para 2 jobs casi idénticos. KISS.
- **Single workflow con matrix [backend, frontend]**: triggers son distintos
  (tags `backend-v*` vs `frontend-v*`), matrix complica.

## Workflow YAML completo

```yaml
name: Build & push domain-frontend image

on:
  push:
    tags:
      - 'frontend-v*'
    paths:
      - 'domain-frontend/**'

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
          images: ghcr.io/nunezlagos/domain-frontend
          tags: |
            type=match,pattern=frontend-v(.*),group=1
            type=raw,value=latest,enable=${{ !contains(github.ref, '-rc') && !contains(github.ref, '-beta') }}

      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: domain-frontend
          file: domain-frontend/Dockerfile
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

## Tiempo esperado de build

- Placeholder actual: nginx + index.html → ~2 min (incl. QEMU emulation arm64).
- Cuando llegue Angular: build stage Node + ng build → ~5-10 min.

## Por qué versionado independiente

Permite releases asincrónicos:
- "Frontend cambió un texto, no toco backend" → `frontend-v1.0.1` solo.
- "Backend agregó endpoint, frontend lo consume" → coordinación de ambos
  tags simultáneos.

Trade-off: el operador debe coordinar versiones cuando hay breaking change
de API. Manejable con API versionada (`/api/v1/*` estable, `/v2/*` para
breaks).
