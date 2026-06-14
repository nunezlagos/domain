# Proposal: issue-19.2-cd-release-binary

## Intención

Release pipeline para `domain-mcp` binary: tag SemVer dispara build multi-OS/arch con goreleaser, artefactos firmados con cosign keyless, changelog convencional auto-generado, install script en gh-pages.

## Scope

**Incluye:**
- `.goreleaser.yml` con builds linux/darwin/windows amd64+arm64
- `.github/workflows/release.yml` con permission id-token write (OIDC para cosign)
- SBOM generado con syft, attachado al release
- `scripts/install.sh` versionado en `gh-pages` branch
- Homebrew tap update opcional (`domain/homebrew-tap`)

**No incluye:**
- Imagen Docker (issue-19.3)
- SDK clients release (REQ-22)

## Enfoque técnico

1. goreleaser v2 con templates de changelog y archive naming consistente
2. cosign keyless con identity GitHub OIDC (no manejamos private keys)
3. Install script con detección uname -s/-m, fallback explícito de arch no soportada
4. Verificación checksum SHA256 + cosign sig en script

## Riesgos

- Install script `curl | sh` es patrón cuestionado por seguridad → documentar verify manual alternativo
- Cosign keyless depende de Sigstore Rekor uptime → degradación tolerable
- Tag accidental dispara release → manual approval opcional en environment

## Testing

- Tag de prueba `v0.0.0-rc1` en branch → release draft generado
- Verify firma: `cosign verify-blob --cert ... --signature ... binary`
- Smoke install script en alpine/ubuntu/macos VM
