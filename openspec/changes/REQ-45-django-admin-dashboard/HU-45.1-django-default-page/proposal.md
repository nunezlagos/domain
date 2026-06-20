# Proposal: HU-45.1-django-default-page

## Intención

Dejar el slot `domain-admin` (que REQ-41 dejó vacío al hacer sunset del Angular) ocupado por un servicio Django mínimo. Es un **placeholder healthcheck-able**: responde 200 en `/` con una página HTML trivial, suficiente para validar el routing de Caddy y el deploy. Las vistas reales (auth, ORM contra el Postgres compartido, CRUD, reporting) entran en HUs siguientes dentro del mismo REQ.

## Scope

**Crea:**
- `services/domain-admin/Dockerfile` — Python 3.12 slim + gunicorn
- `services/domain-admin/docker-compose.yml` — service `domain-admin` en `domain_internal`, expose :80
- `services/domain-admin/.dockerignore` — excluye `__pycache__`, `.pyc`, `*.sqlite3`
- `services/domain-admin/requirements.txt` — Django + gunicorn pinned
- `services/domain-admin/app/manage.py` — entrypoint Django estándar
- `services/domain-admin/app/config/__init__.py` — paquete
- `services/domain-admin/app/config/settings.py` — minimal, sin DB
- `services/domain-admin/app/config/urls.py` — ruta `/` que devuelve HTML
- `services/domain-admin/app/config/wsgi.py` — WSGI app

**No modifica:**
- `services/caddy/Caddyfile` (ya rutea a `domain-admin:80`)
- `services/Makefile` (ya tiene `SVC=admin`)
- `services/install-vps.sh` (no referencia domain-admin explícitamente en el flow principal; queda para HU futura)
- Código del MCP backend

## Enfoque técnico

- Imagen base: `python:3.12-slim` (oficial, chica, bien mantenida)
- Server: `gunicorn` con 2 workers, bind a `0.0.0.0:80` (root OK en container)
- Settings: `DEBUG=False`, `ALLOWED_HOSTS=['*']`, sin `DATABASES['default']` configurado (no se usa DB en esta HU)
- Página `/`: HTML inline con info de versión + link a HU roadmap
- No usar `django-admin startproject` automáticamente — los archivos se commitean a mano para tener control fino

## Riesgos

| Riesgo | Mitigación |
|---|---|
| Root en container expone superficie | Aceptable: container aislado, sin secretos, solo placeholder |
| `gunicorn` con 2 workers desperdicia RAM | Mitigado: `LONG_RUNNING_COUNT` del Makefile ya cuenta solo postgres/minio/mcp/admin/caddy (5 OK) |
| Conflict con el sunset del Angular | Ninguno: el folder `services/domain-admin/` está vacío después del sunset |
| Build local pesado (pip install) | Una vez por build; cache de layers de Docker |

## Testing

- [ ] `docker compose -f services/domain-admin/docker-compose.yml config` parsea OK
- [ ] `make -f services/Makefile help` muestra admin como SVC válido (ya estaba)
- [ ] Manual en VPS: `make restart SVC=admin` levanta, `curl http://<vps>/` devuelve 200 con HTML
- [ ] Sabotaje: cambiar puerto en compose a 81 → Caddy debe devolver 502 → restaurar