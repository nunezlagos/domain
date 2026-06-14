# Design: issue-38.4-caddy-routing-refine

## Decisión arquitectónica

- **Caddy v2 como reverse proxy**: ya está elegido. Justificado: simpleza
  de config, gzip/zstd automáticos, healthcheck declarativo.
- **HTTP plano puerto :80**: decisión cerrada del operador.
- **Routing por path en mismo origin**: evita CORS, único punto de entrada.
- **Sin Caddy admin API**: no se necesita configuración remota dinámica.

## Alternativas descartadas

- **Nginx como reverse proxy**: más maduro pero config más verbosa.
  Caddy 2 línea es nginx 10 líneas.
- **Traefik**: provee service discovery automático Docker, pero
  configuración por labels en cada compose es más distribuida (más difícil
  de auditar de un vistazo).
- **HAProxy**: serie L4/L7 muy potente, pero overkill para 2 upstreams
  estáticos.
- **Subdominios** (`api.<ip>` vs `app.<ip>`): no funcionan con IP plana,
  requerirían sslip.io o dominio. Path-based en mismo origin resuelve.

## Caddyfile final

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

## Compose final

```yaml
services:
  caddy:
    image: caddy:2-alpine
    container_name: domain-caddy
    restart: unless-stopped
    ports:
      - "80:80"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy_data:/data
      - caddy_config:/config
    networks:
      - domain_internal
    healthcheck:
      test: ["CMD", "wget", "-q", "-O", "/dev/null", "http://localhost/"]
      interval: 30s
      timeout: 5s
      retries: 3
      start_period: 10s
    logging:
      driver: json-file
      options:
        max-size: "10m"
        max-file: "3"

networks:
  domain_internal:
    name: domain_internal
    external: true

volumes:
  caddy_data:
    name: domain_caddy_data
  caddy_config:
    name: domain_caddy_config
```

## Por qué `wget` en el healthcheck (y no `caddy validate`)

- `caddy:2-alpine` incluye `wget` por defecto.
- `caddy validate` solo verifica syntaxis del config, no que el server
  esté sirviendo.
- Probe contra `/` end-to-end valida: Caddy listen + routing + frontend
  responde. Si frontend cae, Caddy se marca unhealthy (correcto).

## Por qué `handle` (no `handle_path`)

- `handle_path` strippea el matched prefix; `handle` lo conserva.
- Backend espera el path completo (`/api/v1/orgs`, no `/v1/orgs`).
- Frontend también espera path completo (`/`, `/orgs`, etc.).
- Conclusión: `handle` es la elección correcta.

## Orden de handles importa

Caddy procesa en orden declarado. Los más específicos (`/api/*`, `/mcp*`,
`/healthz`) antes que el catch-all (`handle {}`). Si se invierte, el catch-all
captura todo y los específicos nunca se evalúan.
