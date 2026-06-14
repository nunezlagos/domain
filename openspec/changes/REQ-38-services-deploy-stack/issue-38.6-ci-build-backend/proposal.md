# Proposal: issue-38.6-ci-build-backend

## Intención

Crear un workflow GitHub Actions (`.github/workflows/build-backend.yml`) que
construya la imagen Docker del backend multi-arch (linux/amd64, linux/arm64)
usando `domain-backend/Dockerfile` y la publique en
`ghcr.io/nunezlagos/domain-backend:vX.Y.Z` cada vez que se pushea un tag
`backend-v*`.

## Scope

**Incluye:**
- Archivo `.github/workflows/build-backend.yml` en la rama services con:
  - Trigger: `push: tags: ['backend-v*']`
  - Path filter: `domain-backend/**`
  - Job único `build-and-push` corriendo en `ubuntu-latest`
  - Steps:
    1. checkout
    2. Setup QEMU (para multi-arch)
    3. Setup Docker Buildx
    4. Login a GHCR usando GITHUB_TOKEN
    5. Extract metadata (docker/metadata-action) — genera tags `vX.Y.Z` y `latest`
    6. Build and push (docker/build-push-action) con:
       - context: `domain-backend`
       - file: `domain-backend/Dockerfile`
       - platforms: `linux/amd64,linux/arm64`
       - cache-from: GitHub Actions cache
       - cache-to: same
       - build-args: VERSION, COMMIT, BUILD_TIME inyectados
  - Permissions: `contents: read`, `packages: write`

**No incluye:**
- Tests de Go (eso es de otro workflow CI en main).
- Signing de imágenes (cosign/Sigstore) — futuro opcional.
- Publicación a Docker Hub (solo GHCR).
- Build automático en push a ramas (solo tags).

## Enfoque técnico

1. **Trigger por tag**: workflow YAML:
   ```yaml
   on:
     push:
       tags:
         - 'backend-v*'
   ```

2. **Multi-arch con buildx**:
   ```yaml
   - uses: docker/setup-qemu-action@v3
   - uses: docker/setup-buildx-action@v3
   ```

3. **Metadata extraction**:
   ```yaml
   - id: meta
     uses: docker/metadata-action@v5
     with:
       images: ghcr.io/nunezlagos/domain-backend
       tags: |
         type=match,pattern=backend-v(.*),group=1
         type=raw,value=latest,enable={{is_default_branch}}
   ```
   Esto genera tags como `v1.2.3` y `latest` (si la condición se cumple).

4. **Build push**:
   ```yaml
   - uses: docker/build-push-action@v5
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

5. **Login GHCR**:
   ```yaml
   - uses: docker/login-action@v3
     with:
       registry: ghcr.io
       username: ${{ github.actor }}
       password: ${{ secrets.GITHUB_TOKEN }}
   ```

## Riesgos

- **Cache de buildx grande**: GitHub Actions cache tiene límite 10GB. Mitigar
  con `mode=min` si se llena. Pero `mode=max` da builds más rápidos.
- **Multi-arch lento sin runners ARM nativos**: emulación QEMU es ~2-5x más
  lento que ARM nativo. Build total ~10-15 min para arm64. Mitigación:
  aceptar el costo (libre en GitHub free tier).
- **Permisos GHCR**: el token default tiene write a packages del mismo
  repo/org. Si la imagen se quiere mover a otro namespace, requiere PAT.
  Por ahora `nunezlagos` es el owner del repo → GITHUB_TOKEN suficiente.
- **Tag malformado**: si alguien pushea `backend-vX` sin SemVer, el extract
  metadata podría fallar. Mitigación: el pattern regex `backend-v(.*)` es
  permisivo; mejor pattern: `backend-v\d+\.\d+\.\d+(-.*)?`.
- **CI corre en push a main accidental**: protegido por `on.push.tags`, no
  por branches. Cero riesgo.

## Testing

- Crear tag de prueba `backend-v0.0.0-test`, push, esperar workflow.
- Workflow completa en <15 min.
- Imagen aparece en `https://github.com/nunezlagos/domain/pkgs/container/domain-backend`.
- `docker manifest inspect ghcr.io/nunezlagos/domain-backend:v0.0.0-test`
  muestra `linux/amd64` y `linux/arm64`.
- `docker pull ghcr.io/nunezlagos/domain-backend:v0.0.0-test` exit 0 desde
  cualquier máquina.
- `docker run --rm ghcr.io/nunezlagos/domain-backend:v0.0.0-test --version`
  imprime version inyectada por build-arg.
- Delete del tag de prueba post-validation.
- Después: `docker buildx bake` ya cacheado, build subsecuente <5 min.
