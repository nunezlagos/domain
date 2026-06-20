# Design: HU-45.1-django-default-page

## Decisión arquitectónica

**Django mínimo, sin ORM todavía.** Razones:

1. El sunset de REQ-41 (Angular) dejó `services/domain-admin/` vacío. Re-llenarlo con un servicio que **al menos responda HTTP** valida el routing de Caddy, el healthcheck del Makefile y el deploy end-to-end.
2. El ORM (Django models contra el Postgres compartido) entra en HU-45.2 — no en esta. Esta HU es deliberadamente pequeña: scope mínimo para no mezclar "placeholder vivo" con "lógica de negocio".
3. Django + gunicorn es la elección natural si vamos a usar el ORM de Django más adelante (vs Flask/FastAPI que requerirían SQLAlchemy separado).
4. Stack full-stack (HTML + backend en una sola app Python) matchea el preference del operador.

## Alternativas descartadas

- **Flask + gunicorn**: más liviano pero requiere SQLAlchemy aparte cuando llegue ORM. Migración futura tendría costo.
- **FastAPI**: misma razón + no tiene admin UI built-in (Django admin es un regalo).
- **PHP/Laravel**: mezclar stacks. Decidido ya que no.
- **Static HTML + nginx**: trivial pero pierde el "stack full-stack" como motivo para usar Django.

## Diagrama

```
INTERNET → Caddy :80 ─┬─ /api/* /mcp* /healthz → domain-mcp:8000
                      └─ /*                     → domain-admin:80
                                                    ↑
                                          [gunicorn 2 workers]
                                          [Django default page]
                                          [Sin DB todavía]
```

## TDD plan

N/A — esta HU es infra (docker + hello world). El test real es el sabotaje en el deploy (cambiar puerto → Caddy 502 → restaurar).

## Riesgos y mitigación

| Riesgo | Mitigación |
|---|---|
| Django defaults a `ALLOWED_HOSTS=[]` y rechaza el host del Caddy | `ALLOWED_HOSTS=['*']` en settings |
| `gunicorn` no encuentra la app si paths están mal | `wsgi.py` con `os.environ.setdefault` antes del import |
| `manage.py` apunta a `DJANGO_SETTINGS_MODULE` incorrecto | Usar `config.settings` (path real) |
| Root en container | Aceptable para placeholder; HU futura puede pasar a nonroot con `setcap` o puerto alto + nginx |