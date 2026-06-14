# issue-38.1-backend-dockerfile-refine

**Origen:** `REQ-38-services-deploy-stack`
**Prioridad tentativa:** alta (bloqueante CI)
**Tipo:** infrastructure / docker
**Wave:** 1 (sin dependencias)

## Historia de usuario

**Como** operador del VPS / mantenedor de la imagen Docker del backend
**Quiero** que el `Dockerfile` y `.dockerignore` de `domain-backend/` produzcan
una imagen `ghcr.io/nunezlagos/domain-backend:vX.Y.Z` reproducible, mínima y
publicable
**Para** que CI pueda construir y pushear sin sorpresas, y el VPS pueda hacer
`docker compose pull` y levantar el container en segundos.

## Criterios de aceptación

### Escenario 1: La imagen builda localmente sin error

```gherkin
Dado que estoy en domain-backend/ con Docker buildx disponible
Cuando ejecuto `docker buildx build -t domain-backend:dev --load .`
Entonces el build termina con exit code 0 en <5 min
Y la imagen final pesa <30 MB
Y `docker history domain-backend:dev` muestra solo 2 stages (builder + runtime)
```

### Escenario 2: Imagen tagueada como ghcr.io/nunezlagos/domain-backend

```gherkin
Dado que el Dockerfile fue refinado
Cuando inspecciono los labels OCI
Entonces `org.opencontainers.image.source` == "https://github.com/nunezlagos/domain"
Y existe label que apunta a "ghcr.io/nunezlagos/domain-backend"
Y la imagen NO contiene el label antiguo "ghcr.io/nunezlagos/domain" (singular)
```

### Escenario 3: Runtime distroless funciona

```gherkin
Dado que la imagen está buildeada
Cuando ejecuto `docker run --rm domain-backend:dev healthcheck`
Entonces el comando responde sin necesidad de shell
Y `docker run --rm domain-backend:dev --version` imprime la versión inyectada
Y el container corre como user nonroot (uid != 0)
```

### Escenario 4: .dockerignore excluye lo innecesario

```gherkin
Dado que ejecuto build con BuildKit verbose
Cuando inspecciono el contexto enviado al daemon
Entonces NO incluye .git, .github, .claude, docs/, openspec/, reports/
Y NO incluye archivos *.png, *.svg, *.mmd
Y NO incluye docker-compose.yml local
Y NO incluye *_test.go ni testdata/
Y el contexto pesa <20 MB
```

### Escenario 5: Healthcheck integrado

```gherkin
Dado que el container está corriendo
Cuando Docker engine ejecuta el healthcheck cada 30s
Entonces invoca `/usr/local/bin/domain healthcheck`
Y un container sano reporta status "healthy" después de start_period
Y un container caído reporta status "unhealthy" tras 3 retries
```

## Notas

- El Dockerfile actual (heredado de `main`) ya es multi-stage + distroless.
  Esta HU es **refinamiento**, no rewrite.
- Los binarios incluidos son `domain` y `domain-mcp` (los 2 de runtime).
  Los otros 6 binarios de `cmd/` (lints, schema-drift, etc.) NO van en
  la imagen productiva.
- Sin cambios a la estructura multi-stage ni al base image distroless.
