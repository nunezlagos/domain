# Tasks: issue-19.3-docker-image-publish

- [ ] **img-001**: `Dockerfile` multi-stage distroless static nonroot
- [ ] **img-002**: `.github/workflows/release-image.yml` con buildx + cache GHA + push GHCR+DockerHub
- [ ] **img-003**: Subcomando `domain-mcp healthcheck` para HEALTHCHECK
- [ ] **img-004**: Cosign keyless sign + SBOM syft attest
- [ ] **img-005**: Tags vX.Y.Z, vX, latest
- [ ] **test-001**: docker run smoke /health 200
- [ ] **test-002**: pull arm64 + amd64 ok
- [ ] **test-003**: cosign verify + SBOM
- [ ] **docs-001**: `docs/deploy/docker.md` con ejemplos compose y k8s deployment
- [ ] **docs-002**: Nota sobre debug en distroless (ephemeral container)
