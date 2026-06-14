# Proposal: issue-38.3-frontend-placeholder-scaffold

## Intención

Crear el scaffolding mínimo de `domain-frontend/` (Dockerfile + nginx.conf +
docker-compose.yml + `web/index.html` placeholder) para que el container de
frontend pueda levantar y servir un HTML estático "Coming soon" antes de
que exista código Angular real.

## Scope

**Incluye:**
- `domain-frontend/Dockerfile`:
  - `FROM nginx:1.27-alpine`
  - COPY `nginx.conf` → `/etc/nginx/conf.d/default.conf`
  - COPY `web/` → `/usr/share/nginx/html/`
  - EXPOSE 80
- `domain-frontend/nginx.conf`:
  - server listen 80
  - root /usr/share/nginx/html
  - SPA-style `try_files $uri $uri/ /index.html;`
  - Cache headers diferenciados (assets vs index.html)
  - Gzip on (aunque Caddy ya comprime, defensa en profundidad)
- `domain-frontend/docker-compose.yml`:
  - image: `ghcr.io/nunezlagos/domain-frontend:${DOMAIN_FRONTEND_VERSION:-latest}`
  - container_name: `domain-frontend`
  - restart: unless-stopped
  - expose: ["80"] (sin ports público)
  - networks: [domain_internal] external
  - healthcheck wget http://localhost/ -O /dev/null
- `domain-frontend/web/index.html`:
  - HTML5 doctype, lang="es"
  - <title>Domain — Coming soon</title>
  - Body con texto centrado: "Domain dashboard — coming soon"
  - Footer: "Powered by domain"
  - Sin JS, sin CSS externos, sin imágenes (100% standalone)
  - <meta viewport> para responsive
- Eliminar `domain-frontend/.gitkeep` (ya no hace falta).

**No incluye:**
- Angular scaffolding (eso es REQ-FRONTEND futuro, fuera de scope REQ-38).
- Configuración de Caddy (eso es HU-38.4).
- CI/CD del frontend (eso es HU-38.7).

## Enfoque técnico

1. **nginx-alpine como base**: imagen oficial, ~25MB con html copiado.
   Distroless no sirve porque necesitamos nginx running.
2. **SPA fallback**: `try_files` cubre rutas que el frontend gestiona
   client-side. Para placeholder no aplica pero deja el patrón listo
   para cuando llegue Angular.
3. **Cache strategy**:
   ```nginx
   location / {
     try_files $uri $uri/ /index.html;
   }
   location ~* \.(?:js|css|woff2?|svg|png|jpg|jpeg|gif|ico)$ {
     expires 1y;
     add_header Cache-Control "public, immutable";
   }
   location = /index.html {
     add_header Cache-Control "no-store, must-revalidate";
   }
   ```
4. **Healthcheck en compose**: wget contra `http://localhost/` retorna 200.
5. **Reemplazo futuro**: cuando llegue Angular, solo cambia `web/` —
   ese directorio se reemplaza por `dist/dashboard/` generado por
   `ng build --configuration production`. Dockerfile y nginx.conf
   permanecen.

## Riesgos

- **Wget no está en nginx:alpine**: por default sí está, pero si la imagen
  futura no lo trae, healthcheck rompe. Mitigación: usar curl o caddy-style
  HTTP probe. Alternativa: `nginx -t` (test config) como heuristic.
- **Index.html cacheado agresivo**: si el browser cachea el placeholder y
  luego despleamos Angular, el user no ve el nuevo. Mitigación: `Cache-Control:
  no-store` en index.html resuelve esto.
- **Permisos en nginx**: corre como user nginx (uid 101), no root. Si el
  bind mount tiene permisos restrictivos, no lee los archivos. Mitigación:
  COPY desde el Dockerfile pone los archivos con el owner correcto.
- **Compose sin context build**: usamos image GHCR pinneado, no build local.
  Si la imagen no existe en GHCR cuando el operador hace `make up`, falla.
  Mitigación: HU-38.7 (CI) garantiza que la imagen exista para cada tag.

## Testing

- `docker buildx build -t domain-frontend:dev --load domain-frontend/` exit 0
- `docker images domain-frontend:dev` <30 MB
- `docker run --rm -p 8080:80 domain-frontend:dev &` arranca
- `curl http://localhost:8080/` devuelve HTML con "Coming soon"
- `curl -I http://localhost:8080/index.html` muestra `Cache-Control: no-store`
- `curl http://localhost:8080/nonexistent-route` devuelve el index.html
  (SPA fallback funciona)
- `docker exec <id> nginx -t` válido
- Healthcheck pasa OK después de start_period
- Con red `domain_internal` arriba: `docker compose up -d` levanta y
  Caddy puede alcanzar `http://domain-frontend:80/`
