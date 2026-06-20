# HU-45.1-django-default-page

**Origen:** `REQ-45-django-admin-dashboard`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** operador del VPS
**Quiero** un container `domain-admin` con Django escuchando en :80 detrás de Caddy
**Para** tener un punto de entrada healthcheck-able para el panel admin (luego se reemplaza con vistas reales en HUs siguientes)

## Criterios de aceptación

- Existe `services/domain-admin/Dockerfile` que builda una imagen Python con Django + gunicorn
- Existe `services/domain-admin/docker-compose.yml` que levanta el container en red `domain_internal` exponiendo :80
- Existe `services/domain-admin/app/` con un proyecto Django (`config/`) que responde HTTP 200 en `/`
- La respuesta de `/` muestra una página HTML mínima confirmando que el servicio está vivo
- El comando `make up SVC=admin` levanta el container y queda healthy en `docker ps`
- Caddyfile (sin cambios) sigue ruteando `/` a `domain-admin:80` y la página default es accesible vía `http://<vps>/`
- Sin Co-Authored-By en commits

## Análisis breve

- **Qué pide realmente:** un servicio Django "hello world" dockerizado y hooked a la infra existente (Caddy, network interna, Makefile). NO es un admin real todavía — es el placeholder healthcheck-able.
- **Módulos sospechados:** `services/domain-admin/` (carpeta a crear), `services/Makefile` (sin cambios, ya tiene `SVC=admin`)
- **Riesgos / dependencias:**
  - Puerto 80 dentro del container requiere root (la imagen python:3.12-slim corre como root por default, OK)
  - El init container `domain-migrate` del MCP usa la imagen `domain-mcp:local`, no `domain-admin:local` (no chocan)
- **Esfuerzo tentativo:** S

## Verificación previa

- [x] Revisar codebase (grep) — `services/domain-admin/` ya no existe (borrado en commit `5d03726`), hay que crearlo de cero
- [x] Revisar Caddyfile — `reverse_proxy domain-admin:80` ya está configurado
- [x] Revisar Makefile — `SVC=admin` ya existe y mapea a `domain-admin/docker-compose.yml`
- [x] Revisar git log — REQ-41 (Angular) sunseted en commit `5d03726`, no hay conflicto
- [ ] Probar en ambiente correcto (NO se prueba — regla del proyecto: NUNCA build después de cambios)
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** listo para implementar
- **Evidencia:** Caddyfile + Makefile ya tienen el routing y el SVC preparado, falta solo el container
- **Acción derivada:** crear `services/domain-admin/` con Django mínimo, Dockerfile multi-stage, compose