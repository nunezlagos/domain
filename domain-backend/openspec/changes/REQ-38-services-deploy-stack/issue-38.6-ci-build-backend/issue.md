# issue-38.6-ci-build-backend

**Origen:** `REQ-38-services-deploy-stack`
**Prioridad tentativa:** alta
**Tipo:** ci-cd
**Wave:** 2 (depende de 38.1)

## Historia de usuario

**Como** desarrollador que tagea una nueva versión del backend
**Quiero** que un workflow GitHub Actions construya la imagen multi-arch
(linux/amd64, linux/arm64) usando `domain-backend/Dockerfile` y la publique
en `ghcr.io/nunezlagos/domain-backend:vX.Y.Z`
**Para** que el VPS pueda hacer `docker compose pull` con esa versión y
levantar el container sin tener que buildear localmente.

## Criterios de aceptación

### Escenario 1: Trigger correcto

```gherkin
Dado que existe .github/workflows/build-backend.yml
Cuando push de un tag con formato backend-v*
Entonces el workflow se dispara
Y workflow tag triggers correctos: backend-v1.0.0, backend-v1.2.3-rc1, etc.
Y NO se dispara por push a otras ramas ni por tags que no matcheen
```

### Escenario 2: Path filter (no dispara por cambios irrelevantes)

```gherkin
Dado que un commit toca solo README.md o caddy/Caddyfile
Cuando push del commit
Entonces el workflow NO se dispara (path filter en domain-backend/**)
Y solo cambios en domain-backend/, Dockerfile, .dockerignore disparan
```

### Escenario 3: Build multi-arch

```gherkin
Dado que el workflow corre
Cuando inspecciono el step "Build and push"
Entonces usa docker/build-push-action@v5+
Y platforms: linux/amd64,linux/arm64
Y context: domain-backend
Y file: domain-backend/Dockerfile
Y cache de buildx activado (cache-from + cache-to)
```

### Escenario 4: Push a GHCR con tag correcto

```gherkin
Dado que el workflow completa el build
Cuando inspecciono el resultado en GHCR
Entonces existe ghcr.io/nunezlagos/domain-backend:vX.Y.Z (el tag empujado)
Y existe ghcr.io/nunezlagos/domain-backend:latest (si es la versión más nueva)
Y los manifests incluyen linux/amd64 y linux/arm64
Y la visibilidad de la imagen es public (o privada según preferencia)
```

### Escenario 5: Autenticación segura

```gherkin
Dado que el workflow necesita acceder a GHCR
Cuando hace login
Entonces usa GITHUB_TOKEN (no PAT del usuario)
Y permissions: packages: write
Y el token no se loguea ni queda expuesto
```

### Escenario 6: Inyecta metadata de build

```gherkin
Dado que el Dockerfile soporta ARGs VERSION, COMMIT, BUILD_TIME
Cuando el workflow construye
Entonces pasa --build-arg VERSION=${tag_sin_prefix}
Y --build-arg COMMIT=${github.sha}
Y --build-arg BUILD_TIME=${timestamp_ISO8601}
Y la imagen producida reporta esos valores cuando se ejecuta `domain version`
```

### Escenario 7: Workflow falla rápido en problemas

```gherkin
Dado que el Dockerfile tiene un error de sintaxis
Cuando el workflow corre
Entonces falla en <2 min con mensaje claro
Y la imagen NO se publica
Y el tag queda asociado al fallo en la página de Actions
```

## Notas

- El archivo `.github/workflows/build-backend.yml` vive en la **raíz** de la
  rama services (no dentro de domain-backend/).
- Las pruebas (go test, lint) NO van en este workflow — son de otro workflow
  separado (CI) que probablemente herede de main. Este workflow asume que el
  código ya pasó tests al ser tagueado.
- Multi-arch porque el VPS típico es amd64 pero el dev local puede ser arm64
  (Apple Silicon) y queremos `docker pull` funcione en ambos.
