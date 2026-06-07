# HU-19.2-cd-release-binary

**Origen:** `REQ-19-ci-cd`
**Persona:** platform-engineer, security-officer
**Prioridad tentativa:** media
**Tipo:** infrastructure

## Historia de usuario

**Como** maintainer
**Quiero** publicar binarios multi-arch firmados al pushar tag SemVer
**Para** que usuarios instalen `domain` con `curl | sh` o desde GitHub Releases con verificación

## Criterios de aceptación

### Escenario 1: Tag dispara goreleaser

```gherkin
Dado que pusheo tag `vX.Y.Z` en main
Cuando GitHub Actions ejecuta workflow `release.yml`
Entonces goreleaser builds:
  | OS      | arch          |
  | linux   | amd64, arm64  |
  | darwin  | amd64, arm64  |
  | windows | amd64         |
Y se crea GitHub Release con changelog autogenerado
Y se attachan binaries + checksums.txt + SBOM
```

### Escenario 2: Firma cosign

```gherkin
Dado que goreleaser termina build
Cuando se firman los artifacts
Entonces se generan `.sig` y `.cert` con cosign keyless (OIDC GH Actions)
Y la firma es verificable con `cosign verify-blob`
```

### Escenario 3: Install script

```gherkin
Dado que existe `scripts/install.sh` publicado en gh-pages
Cuando un usuario ejecuta `curl -sSL get.domain.sh | sh`
Entonces detecta OS/arch
Y descarga el binary correspondiente del último release
Y valida checksum + firma cosign
Y lo instala en `/usr/local/bin/domain`
```

### Escenario 4: Changelog convencional

```gherkin
Dado que los commits siguen Conventional Commits
Cuando goreleaser genera changelog
Entonces agrupa por tipo: Features, Fixes, Performance, Docs, BREAKING
Y excluye commits chore/ci/test
```

## Análisis breve

- **Qué pide:** goreleaser + cosign keyless + install script
- **Esfuerzo:** S
- **Riesgos:** secrets management; install script vulnerable si HTTPS endpoint comprometido
