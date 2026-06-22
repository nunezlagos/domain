"""Paquete de settings con seleccion por entorno.

`config.settings` sigue siendo importable y valido (el Dockerfile usa
DJANGO_SETTINGS_MODULE=config.settings). Por defecto carga base + prod.

Seleccion via DJANGO_ENV:
    DJANGO_ENV=prod (default) -> base + prod
    DJANGO_ENV=test           -> base + test

Para tests tambien puede apuntar directo al modulo:
    DJANGO_SETTINGS_MODULE=config.settings.test
"""
import os

from .base import *  # noqa: F401,F403

_env = os.environ.get("DJANGO_ENV", "prod").lower()

if _env == "test":
    from .test import *  # noqa: F401,F403
else:
    from .prod import *  # noqa: F401,F403
