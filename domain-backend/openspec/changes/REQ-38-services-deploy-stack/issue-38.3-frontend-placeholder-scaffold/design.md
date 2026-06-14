# Design: issue-38.3-frontend-placeholder-scaffold

## Decisión arquitectónica

- **Base runtime:** `nginx:1.27-alpine` (~25 MB final).
- **Sin build stage:** el placeholder es HTML estático, no necesita
  `ng build` ni nada. Cuando llegue Angular, se agrega stage builder antes.
- **SPA-style routing en nginx**: `try_files $uri $uri/ /index.html;`.
- **Cache strategy diferenciado**: assets `1y immutable`, index.html
  `no-store`.
- **Compose con image GHCR pinneado**: igual patrón que backend.

## Alternativas descartadas

- **Caddy como server estático**: ya hay Caddy en el stack (reverse proxy).
  Tenerlo doble no aporta. Nginx es el estándar para servir estáticos
  detrás de reverse proxy.
- **Apache**: más pesado, menos común para SPAs.
- **Bind-mount de `web/` desde el host**: rompería el patrón "imagen
  GHCR pinned"; cualquier cambio en `web/` requeriría coordinación
  manual VPS↔repo.
- **Build stage con Node ahora**: innecesario para placeholder. Se agrega
  cuando llegue Angular (en otro HU/REQ futuro).

## Dockerfile final

```dockerfile
FROM nginx:1.27-alpine
LABEL org.opencontainers.image.source="https://github.com/nunezlagos/domain"
LABEL org.opencontainers.image.description="Domain frontend — Angular dashboard SPA (or placeholder)"
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY web/ /usr/share/nginx/html/
EXPOSE 80
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD ["wget", "-q", "-O", "/dev/null", "http://localhost/"]
```

## nginx.conf final

```nginx
server {
  listen 80;
  server_name _;
  root /usr/share/nginx/html;
  index index.html;

  # Compresión adicional (Caddy ya comprime, defensa en profundidad)
  gzip on;
  gzip_types text/plain text/css application/json application/javascript text/xml application/xml;

  # SPA fallback
  location / {
    try_files $uri $uri/ /index.html;
  }

  # Assets versionados: cache largo
  location ~* \.(?:js|css|woff2?|svg|png|jpg|jpeg|gif|ico|webp)$ {
    expires 1y;
    add_header Cache-Control "public, immutable";
  }

  # index.html: nunca cachear
  location = /index.html {
    add_header Cache-Control "no-store, must-revalidate";
    expires 0;
  }
}
```

## index.html placeholder

```html
<!DOCTYPE html>
<html lang="es">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>Domain — Coming soon</title>
  <style>
    body {
      margin: 0;
      min-height: 100vh;
      display: flex;
      flex-direction: column;
      align-items: center;
      justify-content: center;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
      background: #0a0a0a;
      color: #e5e5e5;
    }
    h1 { font-size: 1.5rem; font-weight: 400; margin: 0; }
    footer {
      position: fixed; bottom: 1rem; font-size: 0.8rem; opacity: 0.5;
    }
  </style>
</head>
<body>
  <h1>Domain dashboard — coming soon</h1>
  <footer>Powered by domain</footer>
</body>
</html>
```

## Compose final

```yaml
services:
  domain-frontend:
    image: ghcr.io/nunezlagos/domain-frontend:${DOMAIN_FRONTEND_VERSION:-latest}
    container_name: domain-frontend
    restart: unless-stopped
    expose:
      - "80"
    networks:
      - domain_internal
    healthcheck:
      test: ["CMD", "wget", "-q", "-O", "/dev/null", "http://localhost/"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 5s
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
