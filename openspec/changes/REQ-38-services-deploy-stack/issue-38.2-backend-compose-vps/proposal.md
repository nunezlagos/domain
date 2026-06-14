# Proposal: issue-38.2-backend-compose-vps

## Intención

Reemplazar el `docker-compose.yml` actual de `domain-backend/` (que es el de
DEV LOCAL heredado de main, con PG/MinIO/Adminer/Mailpit embebidos) por un
compose de DEPLOY VPS que define un único service `domain-backend` referenciando
la imagen GHCR pinned, conectado a la red interna compartida.

## Scope

**Incluye:**
- `domain-backend/docker-compose.yml` reescrito con:
  - service único `domain-backend`
  - `image: ghcr.io/nunezlagos/domain-backend:${DOMAIN_BACKEND_VERSION:-latest}`
  - `container_name: domain-backend`
  - `restart: unless-stopped`
  - environment vars apuntando a `postgres:5432` y `minio:9000` (DNS internos)
  - `expose: ["8000"]` (sin ports público — Caddy lo proxyea)
  - `networks: [domain_internal]` external
  - `healthcheck` declarativo
  - `logging` con rotación 10m/3 files
- Cero referencias a postgres, minio, adminer, mailpit (esos están en sus
  propios composes en la rama services).

**No incluye:**
- Migración de la lógica del compose dev local a otro lado (se descarta
  por completo; el dev local del binary usa otro flujo).
- Cambios al Dockerfile (eso es HU-38.1).
- Definición de la network `domain_internal` (la crea Makefile/install.sh).

## Enfoque técnico

1. **Backup mental del compose viejo**: el actual con PG/MinIO/Adminer/Mailpit
   sirve para dev local DEL BINARIO (cuando un dev del proyecto `domain` quiere
   levantar stack local para hackear). Ese flujo NO desaparece — vive como
   "docker-compose.dev.yml" en el repo `domain` rama main, o como simple
   `make dev-up` con un script. Pero NO en la rama services.

2. **Compose nuevo enfocado a producción**:
   ```yaml
   services:
     domain-backend:
       image: ghcr.io/nunezlagos/domain-backend:${DOMAIN_BACKEND_VERSION:-latest}
       container_name: domain-backend
       restart: unless-stopped
       environment:
         DOMAIN_HTTP_PORT: 8000
         DOMAIN_DATABASE_URL: postgres://app_user:${APP_USER_PASSWORD}@postgres:5432/${POSTGRES_DB:-domain}?sslmode=disable
         DOMAIN_S3_ENDPOINT: http://minio:9000
         DOMAIN_S3_ACCESS_KEY: ${MINIO_ROOT_USER}
         DOMAIN_S3_SECRET_KEY: ${MINIO_ROOT_PASSWORD}
         DOMAIN_S3_BUCKET: ${MINIO_DEFAULT_BUCKET:-domain-attachments}
         DOMAIN_S3_REGION: us-east-1
         DOMAIN_S3_FORCE_PATH_STYLE: "true"
       expose:
         - "8000"
       networks:
         - domain_internal
       healthcheck:
         test: ["CMD-SHELL", "/usr/local/bin/domain healthcheck || exit 1"]
         interval: 30s
         timeout: 5s
         retries: 3
         start_period: 15s
       logging:
         driver: json-file
         options:
           max-size: "10m"
           max-file: "3"

   networks:
     domain_internal:
       name: domain_internal
       external: true
   ```

3. **Wait healthy con Make**: como PG/MinIO viven en otros composes, no se
   puede usar `depends_on` cross-compose. El orden lo enforce el Makefile
   (HU-38.8): primero PG (espera healthy), luego MinIO, luego backend.

4. **Renombre del actual**: el compose viejo se sobreescribe. NO se renombra
   a `docker-compose.dev.yml` en este repo porque la rama services no debe
   tener compose de dev local (ese vive en main donde está el código original).

## Riesgos

- **El nombre del container colisiona con uno preexistente**: si en el VPS hay
  un container llamado `domain-backend`, falla. Mitigación: el operador hace
  `docker rm -f domain-backend` antes. install.sh maneja esto vía
  `make down` previo.
- **Postgres no resuelve por DNS**: si los composes no están en la misma red,
  `postgres:5432` no resuelve. Mitigación: enforce que postgres también está
  en `domain_internal` (a refactorear en HU futura si no lo está).
- **Default `latest` en producción**: peligroso. Mitigación: documentar en
  README que se debe pin a vX.Y.Z. CI tag genera versiones específicas.
- **Healthcheck consume CPU**: cada 30s ejecuta el binario. En distroless el
  cold start es <50ms, despreciable. Mitigación: si se vuelve problema, bajar
  a interval 60s.

## Testing

- `docker compose -f domain-backend/docker-compose.yml --env-file .env config -q`
  exit 0 (validación YAML + env interpolation).
- Con red `domain_internal` y PG arriba: `docker compose up -d` levanta el
  container y `docker logs domain-backend` muestra el server escuchando :8000.
- `docker exec domain-backend domain healthcheck` exit 0.
- Sin PG arriba: el container arranca pero healthcheck falla → status
  unhealthy tras 3 retries (correcto).
- `docker network inspect domain_internal | jq '.[0].Containers'` muestra
  domain-backend listed.
- `nc -zv <vps-ip> 8000` desde otra IP falla (puerto NO expuesto público).
- `docker exec -it domain-backend nc -zv postgres 5432` exit 0 (resolución
  DNS interna funciona).
