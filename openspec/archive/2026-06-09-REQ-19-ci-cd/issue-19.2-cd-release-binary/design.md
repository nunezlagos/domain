# Design: issue-19.2-cd-release-binary

## Decisión arquitectónica

**Builder:** goreleaser v2 (estándar Go releases).
**Firma:** cosign keyless OIDC (sin gestión de keys).
**SBOM:** syft (SPDX format).
**Distribución:** GitHub Releases primary; install script en gh-pages secondary; Homebrew tap opcional.

## Alternativas descartadas

- **`go install`:** sin versionado de binary final, sin firma
- **Cosign con private key:** gestión de secrets más compleja, keyless es estado-del-arte
- **GoReleaser Pro:** open source cubre todo

## Estructura

```
.goreleaser.yml          # builds, archives, checksums, signs, release
.github/workflows/release.yml  # trigger on tag v*
scripts/install.sh       # detect OS/arch, download, verify, install
docs/install.md          # alternativas: brew, manual, script
```

## TDD plan

1. Tag rc → release draft con todos los binarios
2. Cosign verify-blob → ok
3. SBOM parse con syft válido
4. Install script en alpine VM → binary funcional
5. Sabotaje: corromper checksum → install script aborta
