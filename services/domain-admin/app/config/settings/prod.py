"""Settings de produccion.

Es el entorno por defecto (DJANGO_ENV no seteado o "prod"). Replica el
comportamiento historico del antiguo config/settings.py: servir HTTP por IP
detras del proxy Caddy, sin forzar HTTPS (opt-in via env en base.py).
"""
from .base import *  # noqa: F401,F403

# Overrides de produccion.
# El default historico ya es HTTP-por-IP (ver SESSION_COOKIE_SECURE/CSRF en
# base.py, que leen del env). No hay overrides duros aqui para no romper el
# deploy actual; este modulo existe como punto de extension explicito.
