# Tasks: issue-38.5-env-example-extended

## Edición del archivo

- [ ] **env-001**: Sumar sección "Versiones de imágenes" antes de Postgres,
      con DOMAIN_BACKEND_VERSION=latest y DOMAIN_FRONTEND_VERSION=latest,
      comentario "Pin a vX.Y.Z en producción".
- [ ] **env-002**: Sumar variable DOMAIN_HTTP_PORT=8000 en sección backend
      con comentario "interno, Caddy lo proxyea en :80".
- [ ] **env-003**: Verificar que TODAS las variables existentes siguen
      presentes con sus defaults originales (cero borrados).
- [ ] **env-004**: Verificar que TODOS los CHANGE_ME placeholders se
      mantienen.
- [ ] **env-005**: Verificar que el comando recomendado para passwords
      sigue al inicio: `openssl rand -base64 48 | tr -d '/+=' | head -c 32`.

## Validación

- [ ] **test-001**: `grep -c '^DOMAIN_BACKEND_VERSION=' .env.example` igual a 1.
- [ ] **test-002**: `grep -c '^DOMAIN_FRONTEND_VERSION=' .env.example` igual a 1.
- [ ] **test-003**: `grep -c '^DOMAIN_HTTP_PORT=' .env.example` igual a 1.
- [ ] **test-004**: `grep -c '^POSTGRES_PASSWORD=' .env.example` igual a 1.
- [ ] **test-005**: `grep -c '^APP_USER_PASSWORD=' .env.example` igual a 1.
- [ ] **test-006**: `grep -c '^APP_ADMIN_PASSWORD=' .env.example` igual a 1.
- [ ] **test-007**: `grep -c '^MINIO_ROOT_PASSWORD=' .env.example` igual a 1.
- [ ] **test-008**: `grep -c '^BACKUP_GPG_PASSPHRASE=' .env.example` igual a 1.
- [ ] **test-009**: `grep -c '^NTFY_TOPIC=' .env.example` igual a 1.
- [ ] **test-010**: `wc -l .env.example` ≤ 30 líneas (sigue compacto).
- [ ] **test-011**: Copiar a `.env`, reemplazar CHANGE_ME con valores random,
      y `docker compose -f postgres/docker-compose.yml --env-file .env config -q`
      exit 0.
- [ ] **test-012**: Mismo test para minio, domain-backend, domain-frontend,
      caddy composes.

## Notas para reviewers

- SOLO se edita `.env.example`. Cero otros archivos.
- NO commitear ningún `.env` real ni con secretos.
- El diff debe ser SUMA de líneas (no remueve nada).
