# Tasks: issue-38.2-backend-compose-vps

## Rewrite del compose

- [ ] **comp-001**: Reemplazar contenido completo de `domain-backend/docker-compose.yml`
      (era dev local con PG/MinIO/Adminer/Mailpit; ahora es deploy VPS único service).
- [ ] **comp-002**: Definir service `domain-backend` con `image:
      ghcr.io/nunezlagos/domain-backend:${DOMAIN_BACKEND_VERSION:-latest}`.
- [ ] **comp-003**: `container_name: domain-backend` (sin guión bajo, alineado
      con convenio del resto).
- [ ] **comp-004**: `restart: unless-stopped`.
- [ ] **comp-005**: Sección `environment:` con todas las DOMAIN_* necesarias
      (DOMAIN_HTTP_PORT, DOMAIN_DATABASE_URL compuesto, DOMAIN_S3_*).
- [ ] **comp-006**: `expose: ["8000"]` (sin ports público).
- [ ] **comp-007**: Network `domain_internal` external true.
- [ ] **comp-008**: Healthcheck con `/usr/local/bin/domain healthcheck`,
      interval 30s, retries 3, start_period 15s.
- [ ] **comp-009**: Logging json-file 10m/3 archivos.

## Validación local

- [ ] **test-001**: `docker compose -f domain-backend/docker-compose.yml --env-file .env config -q`
      exit 0 (valida YAML + interpolación de env).
- [ ] **test-002**: `docker compose -f domain-backend/docker-compose.yml --env-file .env config`
      muestra el compose renderizado con vars resueltas correctamente.
- [ ] **test-003**: Sin red `domain_internal`: `docker compose up -d` falla con
      mensaje claro "network domain_internal not found".
- [ ] **test-004**: Con red creada + PG + MinIO arriba:
      `docker compose -f domain-backend/docker-compose.yml --env-file .env up -d`
      levanta el container.
- [ ] **test-005**: `docker exec domain-backend nc -zv postgres 5432` exit 0
      (resolución DNS interna funciona).
- [ ] **test-006**: `docker exec domain-backend nc -zv minio 9000` exit 0.
- [ ] **test-007**: `docker logs domain-backend --tail 20` muestra "Listening on :8000"
      o equivalente.
- [ ] **test-008**: Healthcheck pasa después de start_period (15s+).
- [ ] **test-009**: `nc -zv <vps-ip> 8000` desde otra IP falla (no expuesto).
- [ ] **test-010**: Sin PG arriba: container arranca pero healthcheck falla →
      status "unhealthy" tras 90s (3 retries × 30s).

## Edge cases

- [ ] **edge-001**: Si `APP_USER_PASSWORD` no está en .env, compose config falla
      con mensaje claro indicando la var faltante.
- [ ] **edge-002**: Si `DOMAIN_BACKEND_VERSION` no está en .env, default `latest`
      aplica (el container puede no existir si CI no publicó imagen).
- [ ] **edge-003**: `docker compose down` detiene el container sin error,
      la red y volumes externos persisten.

## Notas para reviewers

- Cambios SOLO en `domain-backend/docker-compose.yml`.
- El compose viejo era para dev local del binario; se descarta porque la rama
  services es DEPLOY del VPS, no DEV del binario.
- Si un dev de domain quiere stack local para hackear, lo monta desde el repo
  domain (rama main) con `make dev-up` (que ya existe ahí). No es problema
  de esta HU.
