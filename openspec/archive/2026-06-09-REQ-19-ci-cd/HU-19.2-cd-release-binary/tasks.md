# Tasks: HU-19.2-cd-release-binary

- [ ] **rel-001**: `.goreleaser.yml` con builds matrix OS/arch
- [ ] **rel-002**: `.github/workflows/release.yml` con permissions OIDC id-token
- [ ] **rel-003**: Firma cosign keyless de binarios + checksums
- [ ] **rel-004**: SBOM syft attachado a release
- [ ] **rel-005**: Changelog convencional con grupos Features/Fixes/Perf/BREAKING
- [ ] **rel-006**: `scripts/install.sh` con detección OS/arch + verify
- [ ] **rel-007**: Branch `gh-pages` publicando install.sh
- [ ] **rel-008**: Homebrew tap (opcional, segundo sprint)
- [ ] **test-001**: Tag rc → release draft completo
- [ ] **test-002**: Verify firma cosign
- [ ] **test-003**: Smoke install en alpine/ubuntu/macos
- [ ] **docs-001**: `docs/install.md` con todas las opciones
