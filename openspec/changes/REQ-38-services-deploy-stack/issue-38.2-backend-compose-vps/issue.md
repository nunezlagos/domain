# issue-38.2-backend-compose-vps

**Origen:** `REQ-38-services-deploy-stack`
**Prioridad tentativa:** alta
**Tipo:** infrastructure / docker-compose
**Wave:** 1 (sin dependencias)

## Historia de usuario

**Como** operador del VPS
**Quiero** un `domain-backend/docker-compose.yml` que defina cómo se corre el
container del backend en producción (referenciando imagen GHCR, en red interna,
sin puertos públicos)
**Para** poder levantar el backend con `docker compose -f domain-backend/docker-compose.yml up -d`
una vez que Caddy y los servicios de soporte estén arriba.

## Criterios de aceptación

### Escenario 1: Compose referencia imagen GHCR versionada

```gherkin
Dado que el compose fue creado
Cuando inspecciono domain-backend/docker-compose.yml
Entonces el service `domain-backend` tiene image: ghcr.io/nunezlagos/domain-backend:${DOMAIN_BACKEND_VERSION:-latest}
Y NO tiene build: . (no buildea local en producción)
Y restart: unless-stopped está activo
Y container_name: domain-backend
```

### Escenario 2: Sin puertos públicos

```gherkin
Dado que reviso el compose
Cuando busco la sección `ports:`
Entonces NO existe (el container solo expone via `expose:`, no publica al host)
Y `expose: ["8000"]` está presente
Y solo Caddy (otro compose) podrá alcanzarlo vía red interna
```

### Escenario 3: Conectado a red interna compartida

```gherkin
Dado que el compose define networks
Cuando inspecciono la sección
Entonces el service está en `domain_internal` (external: true)
Y NO crea su propia red default
Y puede resolver postgres:5432 y minio:9000 por DNS Docker
```

### Escenario 4: Variables de entorno desde .env

```gherkin
Dado que el container arranca
Cuando lee env vars
Entonces DOMAIN_DATABASE_URL apunta a postgres://app_user:${APP_USER_PASSWORD}@postgres:5432/domain?sslmode=disable
Y DOMAIN_S3_ENDPOINT apunta a http://minio:9000
Y DOMAIN_S3_ACCESS_KEY usa ${MINIO_ROOT_USER}
Y DOMAIN_S3_SECRET_KEY usa ${MINIO_ROOT_PASSWORD}
Y DOMAIN_HTTP_PORT=8000
Y todas las vars vienen del .env del root, no del compose hardcoded
```

### Escenario 5: Healthcheck declarativo

```gherkin
Dado que el compose define healthcheck
Cuando docker engine lo ejecuta
Entonces test: ["CMD-SHELL", "/usr/local/bin/domain healthcheck || exit 1"]
Y interval: 30s, timeout: 5s, retries: 3, start_period: 15s
Y otros services con depends_on pueden esperar service_healthy
```

### Escenario 6: Logging con rotación

```gherkin
Dado que el container está corriendo
Cuando produce logs
Entonces driver: json-file con max-size: 10m y max-file: 3
Y el disco no se llena por logs no rotados
```

## Notas

- El compose existente (heredado de main) es para DEV LOCAL con PG/MinIO embebidos.
  **Esta HU lo reescribe completo** para VPS deploy.
- No incluye PG ni MinIO (esos viven en sus propios composes en la rama services).
- Network `domain_internal` la crea el Makefile vía `ensure-network` (HU-38.8) o
  cualquier otro compose que la declare `external: false` primero.
