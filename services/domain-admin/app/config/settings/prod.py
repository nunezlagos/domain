"""Settings de produccion.

Es el entorno por defecto (DJANGO_ENV no seteado o "prod"). Replica el
comportamiento historico del antiguo config/settings.py: servir HTTP por IP
detras del proxy Caddy, sin forzar HTTPS (opt-in via env en base.py).
"""
from .base import *  # noqa: F401,F403





