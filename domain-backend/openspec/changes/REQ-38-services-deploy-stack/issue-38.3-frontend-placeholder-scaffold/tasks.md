# Tasks: issue-38.3-frontend-placeholder-scaffold

## Archivos

- [ ] **scaf-001**: Eliminar `domain-frontend/.gitkeep` (ya no hace falta).
- [ ] **scaf-002**: Crear `domain-frontend/Dockerfile` (nginx:1.27-alpine,
      copy nginx.conf + web/, expose 80, healthcheck wget).
- [ ] **scaf-003**: Crear `domain-frontend/nginx.conf` con SPA routing,
      cache headers, gzip.
- [ ] **scaf-004**: Crear `domain-frontend/web/` (carpeta).
- [ ] **scaf-005**: Crear `domain-frontend/web/index.html` con HTML5 mínimo,
      texto "Domain dashboard — coming soon", footer "Powered by domain",
      sin assets externos.
- [ ] **scaf-006**: Crear `domain-frontend/docker-compose.yml` con image
      GHCR pinneada, expose 80, network domain_internal external, healthcheck,
      logging.

## Validación local

- [ ] **test-001**: `docker buildx build -t domain-frontend:dev --load domain-frontend/`
      exit 0 en <60s.
- [ ] **test-002**: `docker images domain-frontend:dev` size <30 MB.
- [ ] **test-003**: `docker run --rm -p 8080:80 domain-frontend:dev` arranca
      sin error.
- [ ] **test-004**: `curl http://localhost:8080/` devuelve HTML que contiene
      "Domain dashboard — coming soon".
- [ ] **test-005**: `curl -I http://localhost:8080/index.html` muestra
      `Cache-Control: no-store, must-revalidate`.
- [ ] **test-006**: `curl http://localhost:8080/some/spa/route` devuelve
      el mismo index.html (SPA fallback).
- [ ] **test-007**: Healthcheck del container reporta "healthy" después de
      start_period (5s+).
- [ ] **test-008**: `docker exec <id> nginx -t` valido.
- [ ] **test-009**: HTML valida con W3C validator (sin errores).
- [ ] **test-010**: Página renderiza correctamente en Chrome/Firefox/Safari
      (manual check), responsive en mobile (viewport meta funciona).

## Validación compose

- [ ] **comp-001**: `docker compose -f domain-frontend/docker-compose.yml
      --env-file .env config -q` exit 0.
- [ ] **comp-002**: Con red `domain_internal` y imagen disponible:
      `docker compose up -d` levanta el container.
- [ ] **comp-003**: Desde otro container en la misma red:
      `docker run --rm --network domain_internal curlimages/curl -s
      http://domain-frontend/` devuelve el HTML.
- [ ] **comp-004**: `nc -zv <vps-ip> 80` desde otra IP falla (no expuesto
      direct — pasa por Caddy).

## Notas para reviewers

- Archivos NUEVOS: Dockerfile, docker-compose.yml, nginx.conf, web/index.html.
- `.gitkeep` se borra.
- HTML placeholder es deliberadamente minimal. NO agregar Tailwind, ni
  imágenes, ni JS. Cuando llegue Angular real, todo esto será reemplazado.
- nginx.conf debe ser autocontenido (no requiere includes externos).
