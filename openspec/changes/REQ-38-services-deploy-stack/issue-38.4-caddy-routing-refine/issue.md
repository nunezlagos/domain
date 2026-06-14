# issue-38.4-caddy-routing-refine

**Origen:** `REQ-38-services-deploy-stack`
**Prioridad tentativa:** alta
**Tipo:** infrastructure / reverse-proxy
**Wave:** 1 (sin dependencias)

## Historia de usuario

**Como** operador del VPS
**Quiero** que `caddy/Caddyfile` y `caddy/docker-compose.yml` estén refinados
para rutear correctamente HTTP plano por IP a los 2 servicios internos
(`domain-backend` para /api/* /mcp* /healthz, `domain-frontend` para el resto)
**Para** tener un único punto de entrada público en :80 sin TLS, sin dominio.

## Criterios de aceptación

### Escenario 1: Caddyfile listen en :80 sin TLS

```gherkin
Dado que existe caddy/Caddyfile
Cuando inspecciono la configuración
Entonces la primera línea es `:80 {`
Y NO hay ninguna referencia a TLS, ACME, Let's Encrypt o https
Y NO hay redirect http→https
```

### Escenario 2: Routing por path correcto

```gherkin
Dado que el Caddyfile define handles
Cuando un request entra
Entonces /api/*    → reverse_proxy domain-backend:8000
Y /mcp*            → reverse_proxy domain-backend:8000
Y /healthz         → reverse_proxy domain-backend:8000
Y /metrics         → reverse_proxy domain-backend:8000 (opcional)
Y cualquier otro path → reverse_proxy domain-frontend:80
```

### Escenario 3: Compresión activada

```gherkin
Dado que un cliente envía Accept-Encoding: gzip, zstd
Cuando Caddy responde
Entonces aplica compresión zstd (preferred) o gzip (fallback)
Y los assets HTML/JSON/CSS/JS se sirven comprimidos
```

### Escenario 4: Headers de seguridad básicos

```gherkin
Dado que Caddy responde a un request
Cuando inspecciono los headers
Entonces X-Content-Type-Options: nosniff
Y Referrer-Policy: strict-origin-when-cross-origin
Y X-Frame-Options: DENY (opcional pero recomendado)
```

### Escenario 5: Healthcheck del propio Caddy

```gherkin
Dado que el compose define healthcheck
Cuando docker engine lo ejecuta
Entonces test: ["CMD", "wget", "-q", "-O", "/dev/null", "http://localhost/"]
Y o equivalente con `caddy validate` desde el container
```

### Escenario 6: Network compartida + único port público

```gherkin
Dado que reviso caddy/docker-compose.yml
Cuando inspecciono networks y ports
Entonces network: domain_internal (external: true)
Y ports: ["80:80"] (único port público de TODO el stack)
Y NO publica 443 (sin TLS por decisión)
Y volumes: caddy_data y caddy_config (persistencia)
```

### Escenario 7: Logs accesibles via docker logs

```gherkin
Dado que llega tráfico
Cuando ejecuto `docker logs domain-caddy --tail 50`
Entonces veo logs estructurados con timestamp, método, path, status, duration
Y log driver: json-file con max-size 10m, max-file 3
```

## Notas

- Las carpetas `caddy/Caddyfile` y `caddy/docker-compose.yml` ya existen
  (creadas en commit 336e714). Esta HU las refina.
- Sin TLS: decisión explícita del operador (no hay dominio, no se usará HTTPS
  falso ni sslip.io). HTTP plano por IP es definitivo "por ahora".
- Cuando llegue dominio en el futuro, cambio será de 1 línea (issue futuro,
  fuera de scope de REQ-38).
