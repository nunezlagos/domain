# HU-19.3-docker-image-publish

**Origen:** `REQ-19-ci-cd`
**Persona:** platform-engineer
**Prioridad tentativa:** media
**Tipo:** infrastructure

## Historia de usuario

**Como** operador
**Quiero** una imagen Docker oficial `domain/domain-mcp:vX.Y.Z` multi-arch publicada en GHCR y Docker Hub
**Para** deployar Domain en Kubernetes/Compose sin compilar localmente

## Criterios de aceptación

### Escenario 1: Imagen multi-arch publicada en tag

```gherkin
Dado que pusheo tag `vX.Y.Z`
Cuando se ejecuta workflow `release-image.yml`
Entonces se buildea y pushea imagen multi-arch:
  | registry              | tag                |
  | ghcr.io/domain/domain-mcp | vX.Y.Z, vX, latest |
  | docker.io/domain/domain-mcp | vX.Y.Z, vX, latest |
Y la imagen funciona en `linux/amd64` y `linux/arm64`
```

### Escenario 2: Imagen distroless / mínima

```gherkin
Dado que el Dockerfile usa multi-stage
Cuando inspecciono la imagen final
Entonces base es `gcr.io/distroless/static-debian12:nonroot`
Y tamaño < 50 MB
Y no contiene shell ni package manager
Y corre como UID 65532 (nonroot)
```

### Escenario 3: Firma y SBOM

```gherkin
Dado que la imagen está pushada
Cuando ejecuto `cosign verify ghcr.io/domain/domain-mcp:vX.Y.Z`
Entonces la firma keyless valida con identity GH Actions
Y existe attestation SBOM accesible vía `cosign download sbom`
```

### Escenario 4: Healthcheck embebido

```gherkin
Dado que la imagen tiene `HEALTHCHECK`
Cuando corre en compose/k8s
Entonces el healthcheck invoca `/health` endpoint
Y status reporta healthy/unhealthy correctamente
```

## Análisis breve

- **Qué pide:** Dockerfile distroless multi-stage + buildx multi-arch + cosign + SBOM
- **Esfuerzo:** S
- **Riesgos:** distroless sin shell dificulta debug en prod → documentar workaround con ephemeral container
