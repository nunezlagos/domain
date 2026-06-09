# Design: HU-19.3-docker-image-publish

## Decisión arquitectónica

**Base final:** `gcr.io/distroless/static-debian12:nonroot`
**Buildx:** GitHub Actions con cache `type=gha`
**Registries:** GHCR (primary), Docker Hub (mirror)
**Sign+SBOM:** cosign keyless + syft attestation

## Alternativas descartadas

- **Alpine:** glibc vs musl produce edge cases; distroless static cubre Go puro
- **Scratch:** sin CA certs, sin tzdata — distroless static incluye ambos
- **Solo GHCR:** Docker Hub mantiene mindshare; doble push barato

## Dockerfile

```dockerfile
# syntax=docker/dockerfile:1.7
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags="-s -w -X main.Version=$VERSION" \
    -o /out/domain-mcp ./cmd/domain-mcp

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/domain-mcp /usr/local/bin/domain-mcp
USER nonroot
EXPOSE 8080 9090
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD ["/usr/local/bin/domain-mcp", "healthcheck"]
ENTRYPOINT ["/usr/local/bin/domain-mcp"]
CMD ["server"]
```

## TDD plan

1. `docker run` con DOMAIN_DATABASE_URL → /health 200
2. Multi-arch: pull en mac arm64 y linux amd64 → run ok
3. cosign verify ok
4. SBOM contiene módulos esperados
5. Sabotaje: cambiar a base con shell → review fail por convención
