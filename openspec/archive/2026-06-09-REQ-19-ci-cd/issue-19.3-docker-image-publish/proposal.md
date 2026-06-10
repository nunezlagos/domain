# Proposal: issue-19.3-docker-image-publish

## Intención

Publicar imagen Docker oficial `domain/domain-mcp` multi-arch (amd64/arm64) basada en distroless, firmada con cosign, con SBOM, en GHCR y Docker Hub en cada release.

## Scope

**Incluye:**
- `Dockerfile` multi-stage: builder Go → distroless final
- `.github/workflows/release-image.yml` con buildx, cache GitHub Actions, push multi-arch
- Cosign keyless image sign + attest SBOM
- HEALTHCHECK invocando /health
- Tags: `vX.Y.Z`, `vX`, `latest`

**No incluye:**
- Helm chart (otra HU futura)
- Operator Kubernetes
- Dev image (la del compose dev usa fresh build local)

## Enfoque técnico

1. Builder stage `golang:1.23-alpine` con `CGO_ENABLED=0`, ldflags strip
2. Final stage `gcr.io/distroless/static-debian12:nonroot`
3. `docker/build-push-action@v6` con platforms `linux/amd64,linux/arm64`
4. Cache `type=gha` para layers
5. Cosign sign con OIDC + SBOM attestation con syft action

## Riesgos

- Distroless no permite shell debug → documentar `kubectl debug` con ephemeral
- Multi-arch build lento sin cache → cache GHA crítico
- Docker Hub rate limits → fallback a GHCR primary

## Testing

- `docker run` la imagen → /health responde 200
- Verify cosign + SBOM
- Test en compose dev sustituyendo imagen local por la oficial
