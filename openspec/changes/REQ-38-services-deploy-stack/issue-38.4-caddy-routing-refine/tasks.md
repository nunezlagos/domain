# Tasks: issue-38.4-caddy-routing-refine

## Caddyfile

- [ ] **cad-001**: Confirmar primera línea `:80 {` (sin TLS, sin dominio).
- [ ] **cad-002**: Handle `/api/*` → `reverse_proxy domain-backend:8000`.
- [ ] **cad-003**: Handle `/mcp*` → `reverse_proxy domain-backend:8000`.
- [ ] **cad-004**: Handle `/healthz` → `reverse_proxy domain-backend:8000`.
- [ ] **cad-005**: Catch-all `handle { reverse_proxy domain-frontend:80 }`.
- [ ] **cad-006**: `encode zstd gzip` activado.
- [ ] **cad-007**: Headers de seguridad: X-Content-Type-Options, Referrer-Policy.

## Compose

- [ ] **comp-001**: Confirmar image `caddy:2-alpine`.
- [ ] **comp-002**: `container_name: domain-caddy`, `restart: unless-stopped`.
- [ ] **comp-003**: `ports: ["80:80"]` (único port público del stack).
- [ ] **comp-004**: Volume Caddyfile read-only.
- [ ] **comp-005**: Volumes named `caddy_data` y `caddy_config` (persistencia).
- [ ] **comp-006**: Network `domain_internal` external true.
- [ ] **comp-007**: Healthcheck `wget` contra `/` con interval 30s.
- [ ] **comp-008**: Logging json-file 10m/3.

## Validación local

- [ ] **test-001**: `docker compose -f caddy/docker-compose.yml --env-file .env config -q`
      exit 0.
- [ ] **test-002**: `caddy validate --config caddy/Caddyfile --adapter caddyfile`
      exit 0 (o equivalente con `docker run --rm -v $(pwd)/caddy/Caddyfile:/etc/caddy/Caddyfile caddy:2-alpine caddy validate --config /etc/caddy/Caddyfile`).
- [ ] **test-003**: Levantar Caddy + backend + frontend en `domain_internal`:
      `make up`.
- [ ] **test-004**: `curl http://localhost/` (en VPS) devuelve HTML del frontend.
- [ ] **test-005**: `curl http://localhost/api/v1/orgs` devuelve respuesta del
      backend (200 o 401 si requiere auth).
- [ ] **test-006**: `curl http://localhost/healthz` devuelve 200.
- [ ] **test-007**: `curl http://localhost/mcp` devuelve respuesta del MCP HTTP
      handler.
- [ ] **test-008**: `curl -I http://localhost/` muestra:
      - `X-Content-Type-Options: nosniff`
      - `Referrer-Policy: strict-origin-when-cross-origin`
- [ ] **test-009**: `curl -H "Accept-Encoding: zstd" -I http://localhost/`
      muestra `Content-Encoding: zstd`.
- [ ] **test-010**: Healthcheck del container reporta "healthy" tras
      start_period.
- [ ] **test-011**: `docker logs domain-caddy --tail 20` muestra requests con
      status code, path, duration.
- [ ] **test-012**: `nc -zv <vps-ip> 80` desde otra IP exit 0.
- [ ] **test-013**: `nc -zv <vps-ip> 443` desde otra IP falla (no TLS).

## Edge cases

- [ ] **edge-001**: Backend caído: `curl http://localhost/api/...` devuelve
      502 Bad Gateway (Caddy reporta correctamente).
- [ ] **edge-002**: Frontend caído: `curl http://localhost/` devuelve 502.
- [ ] **edge-003**: Path no matcheado por específicos cae al frontend:
      `curl http://localhost/random` devuelve HTML.
- [ ] **edge-004**: Reload de Caddy sin downtime: edit Caddyfile + 
      `docker exec domain-caddy caddy reload --config /etc/caddy/Caddyfile`
      → reverse proxy sigue OK.

## Notas para reviewers

- Archivos en `caddy/` (ya existen los esqueletos del commit 336e714, esta
  HU los refina).
- Cero referencias a TLS, https, ACME, Let's Encrypt, dominio. Decisión
  explícita del operador.
- Si en el futuro cambia (dominio + TLS), Caddyfile cambia 1 línea (`:80 {`
  → `midominio.cl {`). No es alcance de esta HU.
