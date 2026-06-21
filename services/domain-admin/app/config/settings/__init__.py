"""Paquete de settings con selección por entorno.

`config.settings` sigue siendo importable y válido (el Dockerfile usa
DJANGO_SETTINGS_MODULE=config.settings). Por defecto carga base + prod.

Selección via DJANGO_ENV:
    DJANGO_ENV=prod (default) -> base + prod
    DJANGO_ENV=test           -> base + test

Para tests también podés apuntar directo al módulo:
    DJANGO_SETTINGS_MODULE=config.settings.test
"""
import os

from .base import *  # noqa: F401,F403

_env = os.environ.get("DJANGO_ENV", "prod").lower()

if _env == "test":
    from .test import *  # noqa: F401,F403
else:
    from .prod import *  # noqa: F401,F403
