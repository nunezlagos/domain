# Proposal: issue-38.4-caddy-routing-refine

## Intención

Refinar el `caddy/Caddyfile` y `caddy/docker-compose.yml` ya creados (commit
336e714) para asegurar routing correcto por path entre `domain-backend:8000`
(API + MCP + healthz) y `domain-frontend:80` (resto), con headers de seguridad
mínimos y healthcheck del propio Caddy.

## Scope

**Incluye:**
- `caddy/Caddyfile`:
  - Confirmar `:80 {` (sin TLS, sin dominio)
  - Handles por path:
    - `/api/*` → `reverse_proxy domain-backend:8000`
    - `/mcp*` → `reverse_proxy domain-backend:8000` (con o sin slash)
    - `/healthz` → `reverse_proxy domain-backend:8000`
    - `/` (resto) → `reverse_proxy domain-frontend:80`
  - `encode zstd gzip`
  - Headers de seguridad básicos:
    - `X-Content-Type-Options "nosniff"`
    - `Referrer-Policy "strict-origin-when-cross-origin"`
- `caddy/docker-compose.yml`:
  - image: `caddy:2-alpine`
  - container_name: `domain-caddy`
  - ports: `["80:80"]`
  - volumes:
    - Caddyfile read-only
    - caddy_data + caddy_config persistentes (named volumes)
  - networks: [domain_internal] external
  - restart: unless-stopped
  - healthcheck declarativo
  - logging con rotación

**No incluye:**
- TLS/Let's Encrypt/sslip.io/Cloudflare Tunnel (decisión cerrada del operador:
  solo IP plano).
- Rate limiting o autenticación a nivel proxy (lo hace el backend).
- Métricas Prometheus de Caddy (puede agregarse en HU futura).

## Enfoque técnico

1. **Caddyfile mínimo y declarativo**:
   ```caddy
   :80 {
     handle /api/* {
       reverse_proxy domain-backend:8000
     }
     handle /mcp* {
       reverse_proxy domain-backend:8000
     }
     handle /healthz {
       reverse_proxy domain-backend:8000
     }
     handle {
       reverse_proxy domain-frontend:80
     }

     encode zstd gzip

     header {
       X-Content-Type-Options "nosniff"
       Referrer-Policy "strict-origin-when-cross-origin"
     }
   }
   ```

2. **Por qué `handle` (no `handle_path`)**: `handle` matchea por path SIN
   strippear el prefix. El backend recibe el path completo (`/api/v1/orgs`),
   no necesita reescribir.

3. **Order de handles importa**: Caddy v2 procesa handles en orden declarado
   para la misma directiva. Los más específicos (`/api/*`, `/mcp*`, `/healthz`)
   van primero, el catch-all (`handle` sin matcher) va último.

4. **Compose con healthcheck**:
   ```yaml
   healthcheck:
     test: ["CMD", "wget", "-q", "-O", "/dev/null", "http://localhost/"]
     interval: 30s
     timeout: 5s
     retries: 3
     start_period: 10s
   ```
   Esto va a fallar si frontend está abajo. Alternativa: probe contra
   `/healthz` (que va al backend). Decisión: probe al `/` que devuelve
   200 si frontend está OK; si frontend está caído, Caddy se marca
   unhealthy, lo cual es correcto.

5. **Volúmenes persistentes**: aunque no haya TLS, Caddy mantiene state
   (locks, certs internos si los hubiera). Los volumes evitan re-fetch
   en restarts.

## Riesgos

- **Orden de start vs dependencias**: si Caddy arranca antes que backend/
  frontend, sus reverse_proxy fallan al primer intento. Mitigación: Caddy
  reintenta automáticamente y los upstreams aparecen en <30s. Healthcheck
  no se marca healthy hasta que frontend responda.
- **Path /mcp* matchea /mcpsomething**: el wildcard `*` después de `/mcp`
  matchea `/mcpsomething`. Si el backend no tiene esa ruta, devuelve 404.
  Mitigación: cambiar a `/mcp/*` si se quiere ser estricto, o aceptar el
  comportamiento.
- **Compresión zstd no soportada por todos los clientes**: Caddy negocia
  `Accept-Encoding`. Fallback a gzip o identity automático.
- **Wget no presente en caddy:alpine**: NO está. Alternativa: usar
  `caddy validate --config /etc/caddy/Caddyfile` como healthcheck, o
  cambiar la imagen base, o usar `CMD-SHELL` con `nc -z localhost 80`.

## Testing

- `docker compose -f caddy/docker-compose.yml --env-file .env config -q`
  exit 0
- Con red + backend + frontend arriba: `docker compose up -d caddy` levanta OK
- `curl http://localhost/` (en VPS) → HTML del frontend placeholder
- `curl http://localhost/api/v1/orgs` → JSON del backend (o 401 si requiere auth)
- `curl http://localhost/healthz` → 200 OK (proxy al backend)
- `curl http://localhost/mcp` → response del MCP HTTP handler del backend
- `curl -I http://localhost/` muestra:
  - `X-Content-Type-Options: nosniff`
  - `Referrer-Policy: strict-origin-when-cross-origin`
  - `Content-Encoding: gzip` o `zstd` (si Accept-Encoding lo soporta)
- Healthcheck del container reporta "healthy" tras start_period
- `docker logs domain-caddy --tail 10` muestra requests procesados con
  status, path, duration
- `nc -zv <vps-ip> 80` desde otra IP exit 0 (único puerto público)
- `nc -zv <vps-ip> 443` desde otra IP falla (sin TLS, sin :443)
