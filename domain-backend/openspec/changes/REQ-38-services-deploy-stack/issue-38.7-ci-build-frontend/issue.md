# issue-38.7-ci-build-frontend

**Origen:** `REQ-38-services-deploy-stack`
**Prioridad tentativa:** media
**Tipo:** ci-cd
**Wave:** 2 (depende de 38.3)

## Historia de usuario

**Como** desarrollador que tagea una nueva versión del frontend
**Quiero** un workflow GitHub Actions análogo al backend pero para
`domain-frontend/`, que construya multi-arch y publique en
`ghcr.io/nunezlagos/domain-frontend:vX.Y.Z`
**Para** que UI y backend evolucionen con versionado independiente sin
ceremonia extra.

## Criterios de aceptación

### Escenario 1: Trigger por tag frontend-v*

```gherkin
Dado que existe .github/workflows/build-frontend.yml
Cuando push de un tag con formato frontend-v*
Entonces el workflow se dispara
Y workflow tag triggers: frontend-v1.0.0, frontend-v2.5.0, etc.
```

### Escenario 2: Path filter en domain-frontend/

```gherkin
Dado que un commit toca solo domain-backend/internal/foo.go
Cuando push del commit
Entonces el workflow build-frontend.yml NO se dispara
Y solo cambios en domain-frontend/** lo disparan
```

### Escenario 3: Build con context domain-frontend

```gherkin
Dado que el workflow corre
Cuando ejecuta el build
Entonces context: domain-frontend
Y file: domain-frontend/Dockerfile
Y platforms: linux/amd64,linux/arm64
Y push a ghcr.io/nunezlagos/domain-frontend:${tag}
```

### Escenario 4: Imagen final pesa <30 MB

```gherkin
Dado que el frontend es nginx + index.html placeholder
Cuando inspecciono la imagen publicada
Entonces tamaño total <30 MB
Y la imagen reporta linux/amd64 + linux/arm64 manifests
```

### Escenario 5: Misma autenticación que backend

```gherkin
Dado que el workflow necesita acceder a GHCR
Cuando hace login
Entonces usa GITHUB_TOKEN
Y permissions: packages: write
```

## Notas

- Misma estructura que 38.6 pero apuntando a `domain-frontend/`.
- Cuando llegue Angular real, este workflow seguirá funcionando: el
  Dockerfile internamente buildeará `ng build --configuration production`
  como primera stage, copiará dist/ a nginx en la stage final.
- Tagging separado (frontend-v vs backend-v) permite releases independientes
  pero requiere coordinación humana cuando hay breaking change en API.
