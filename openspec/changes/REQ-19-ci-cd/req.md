# REQ-19-ci-cd: CI/CD del repo Domain: pipelines GitHub Actions para lint, test, build, release y publicación de imagen Docker oficial.

**Estado:** activo
**Creado:** 2026-06-06

## Descripción

Pipelines de integración y entrega continua para el binario `domain-mcp` y su imagen Docker. CI ejecuta lint + tests unitarios + integración en cada PR; CD publica releases versionados con SemVer y empuja imágenes Docker firmadas al registry.

## Criterios de éxito

- CI corre lint (`golangci-lint`), `go vet`, `go test ./...` y cobertura en cada push y PR contra main
- Tests de integración con testcontainers (Postgres, MinIO) ejecutados en CI con cache
- CD release: tag SemVer dispara build multi-arch (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64) con goreleaser
- Imagen Docker `domain/domain-mcp:vX.Y.Z` publicada en GHCR y Docker Hub, firmada con cosign, SBOM publicada
- Cache de dependencias Go y Docker layers para reducir tiempos a <5 min en PR

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| HU-19.1-ci-lint-test | proposed | GitHub Actions: lint, vet, unit tests, integration tests con testcontainers, coverage |
| HU-19.2-cd-release-binary | proposed | goreleaser multi-arch en tag, changelog auto, GitHub Release con artifacts firmados |
| HU-19.3-docker-image-publish | proposed | Imagen Docker oficial publicada en GHCR + Docker Hub, firma cosign, SBOM |
