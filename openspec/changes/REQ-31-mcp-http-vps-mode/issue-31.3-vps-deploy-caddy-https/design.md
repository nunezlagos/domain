# Design: issue-31.3-vps-deploy-caddy-https

## Contexto

Domain pasa de local a VPS Contabo. El deploy tiene que:
- Exponer `https://api.tudominio.com` con TLS válido.
- Mantener Postgres y MinIO en localhost (NO accesibles desde
  internet).
- Ser idempotente y recoverable.
- Backup nightly con retención.

Caddy fue elegido por sobre nginx+certbot por su auto-TLS (ACME
nativo) y por su Caddyfile minimalista (3-5 líneas vs 50+ de
nginx). Tradeoff: comunidad más chica que nginx, pero para nuestro
caso (reverse proxy + TLS) es la mejor opción.

## Decisión arquitectónica

**Estrategia:** script bash + docker compose + Caddyfile + cron
job, todo versionado en `deploy/contabo/`.

1. **Estructura de archivos:**
   ```
   deploy/contabo/
     setup.sh              ← entrypoint (este corre el operador)
     lib/
       common.sh           ← helpers: log, error, check_root
       checks.sh           ← pre-flight: DNS, docker, OS
       secrets.sh          ← genera secrets si no existen
       caddy.sh            ← escribe Caddyfile
       compose.sh          ← docker compose up/down
     compose/
       docker-compose.yml  ← production compose (server, postgres, minio, caddy)
     caddy/
       Caddyfile.template  ← template con dominio variable
     backup/
       backup.sh           ← pg_dump + retention
       backup.cron         ← línea crontab
     README.md             ← how-to
   ```

2. **Caddyfile (template):**
   ```caddy
   {$DOMAIN:api.tudominio.com} {
       reverse_proxy localhost:8000
       encode gzip
       header {
           Strict-Transport-Security "max-age=31536000; includeSubDomains"
           X-Content-Type-Options "nosniff"
           X-Frame-Options "DENY"
       }
   }
   ```

3. **docker-compose.yml (production):**
   ```yaml
   services:
     caddy:
       image: caddy:2
       ports: ["443:443", "80:80"]  # solo HTTPS expuesto
       volumes:
         - ./caddy/Caddyfile:/etc/caddy/Caddyfile:ro
         - caddy_data:/data
     domain:
       build: .
       environment:
         - DOMAIN_HTTP_BIND=127.0.0.1
         - DOMAIN_HTTP_PORT=8000
         - DOMAIN_DATABASE_URL=postgres://...
       # NO expone puertos
     postgres:
       image: postgres:15
       # NO expone puertos al host
     minio:
       image: minio/minio
       # NO expone puertos al host
   volumes:
     caddy_data:
   ```

4. **DNS pre-flight:** `dig +short $DOMAIN` debe retornar la IP
   del VPS. Si no, abortar con "DNS not propagated" (Caddy
   fallaría el ACME de todos modos).

5. **Secrets:** `secrets.sh` genera con `openssl rand -base64 32`:
   `DOMAIN_MASTER_KEY`, `POSTGRES_PASSWORD`, `MINIO_ROOT_PASSWORD`,
   `JWT_SECRET`. Guarda en `/opt/domain/.env` (chmod 600). Si el
   archivo ya existe, no regenera (preserva).

6. **Backup nightly:** `backup.sh` corre `docker exec postgres
   pg_dump -U postgres domain | gzip > /backups/db-$(date
   +%Y%m%d).sql.gz`. Retiene últimos 7. Hook a cron via
   `backup.cron` que el setup.sh instala.

7. **Health check del deploy:** post `docker compose up`, esperar
   30s y curl `/health` desde localhost. Si no responde,
   `docker compose logs` y exit !=0.

## Alternativas descartadas

| Alt | Idea | Por qué se descarta |
|-----|------|---------------------|
| A | nginx + certbot | Más config (50+ líneas nginx.conf + certbot cron + reload). Caddy es 1 línea. |
| B | Traefik | Similar a Caddy pero más complejo (labels de docker). Caddyfile es más legible. |
| C | Cloudflare Tunnel | Vendor lock-in. No es necesario para nuestro caso (TLS directo de Let's Encrypt). |
| D | Kubernetes (k3s) | Overkill para 1 VPS. El target es 1 server con 3 servicios. Compose es suficiente. |
| E | Systemd nativo sin Docker | El repo ya tiene Docker como abstracción. Mantener Docker = dev y prod iguales. |

## Por qué Caddy + compose gana

- **Caddy:** auto-TLS, 1 línea de config, headers de seguridad
  built-in.
- **Compose:** el dev usa `docker compose` local; prod usa la
  misma imagen, mismos env vars. Zero "works on my machine".
- **No expone Postgres/MinIO:** simple: no hay `ports:` en el
  compose para esos services.
- **Idempotente:** re-correr `setup.sh` no rompe nada (chequea
  existencia antes de crear).

## Detalle de implementación

- `setup.sh` es el entrypoint. Flujo:
  1. `source lib/common.sh lib/checks.sh lib/secrets.sh lib/caddy.sh lib/compose.sh`.
  2. Check root, OS Ubuntu 24.04, docker, DNS.
  3. Crear `/opt/domain/`, clonar repo (si no existe).
  4. `secrets.sh:ensure_secrets /opt/domain/.env`.
  5. `caddy.sh:write_caddyfile /opt/domain/caddy/Caddyfile $DOMAIN`.
  6. `compose.sh:up /opt/domain`.
  7. Wait + health check.
  8. `backup.sh:install_cron`.
  9. Print summary.

- Documentación en `deploy/contabo/README.md` con: prerequisites,
  cómo apuntar DNS, cómo correr setup.sh, cómo hacer rollback,
  cómo rotar secrets, troubleshooting común.

## Riesgos

- **R1:** El script de deploy se rompe si Caddy no logra el ACME.
  **Mitigación:** DNS pre-flight + retry logic con backoff.
- **R2:** Postgres muere y el backup no sirve. **Mitigación:**
  health check verifica último backup <26h, alerta vía email
  (puede ser simple mail en stderr; integración con SMTP en
  issue 34.3).
- **R3:** Script de deploy tiene bug y deja el VPS en estado
  inconsistente. **Mitigación:** `setup.sh --rollback` que
  docker compose down + restore de secrets desde backup.

## Sabotaje test (referencia)

Cambiar `localhost:8000` a `localhost:9999` en Caddyfile template
(sabotaje) → correr setup.sh con dominio de prueba → health check
externo retorna 502 → el script de health check DEBE detectar y
reportar error → restaurar puerto → verde.
