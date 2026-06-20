# Tasks: HU-45.1-django-default-page

## Scaffold

- [ ] Crear `services/domain-admin/Dockerfile` (python:3.12-slim + gunicorn)
- [ ] Crear `services/domain-admin/.dockerignore`
- [ ] Crear `services/domain-admin/requirements.txt` (Django + gunicorn pinned)
- [ ] Crear `services/domain-admin/docker-compose.yml`

## Django app

- [ ] Crear `services/domain-admin/app/manage.py`
- [ ] Crear `services/domain-admin/app/config/__init__.py` (vacío)
- [ ] Crear `services/domain-admin/app/config/settings.py` (DEBUG=False, ALLOWED_HOSTS=*, sin DATABASES)
- [ ] Crear `services/domain-admin/app/config/urls.py` (path `/` con HttpResponse HTML)
- [ ] Crear `services/domain-admin/app/config/wsgi.py` (aplicación WSGI)

## Verificación

- [ ] `docker compose -f services/domain-admin/docker-compose.yml config` parsea OK
- [ ] `grep -rn domain-mcp services/admin` debe ser 0 (no leak del rename)
- [ ] Commit dedicado: `feat(services): add domain-admin django placeholder`

## Cierre

- [ ] Sin Co-Authored-By
- [ ] Working tree limpio post-commit