"""HU-47.1: Django settings con login simple.

Cero DB (signed cookie sessions). Credenciales master via env vars.

HISTORIAL:
- HU-45.1: placeholder estático
- HU-47.1: login simple single-user (este)

PRÓXIMOS:
- HU-45.2: ORM (Django models contra Postgres compartido) + User multi-user
"""
import os
from pathlib import Path

BASE_DIR = Path(__file__).resolve().parent.parent

SECRET_KEY = "hu-47.1-rotate-me-before-prod-3f8a9b2c1d4e5f6a"
DEBUG = False
ALLOWED_HOSTS = ["*"]

# HU-47.1: minimal apps. django.contrib.sessions requiere DB accesible
# (su ready() hace SELECT 1). Usamos SQLite file-based, no necesita servicio.
INSTALLED_APPS = [
    "django.contrib.sessions",      # HU-47.1: para SessionMiddleware
    "django.contrib.staticfiles",
]

# HU-47.1: solo SessionMiddleware (sin auth middleware porque no usamos
# django.contrib.auth.User). Cookie firmada por SECRET_KEY.
MIDDLEWARE = [
    "django.contrib.sessions.middleware.SessionMiddleware",
]

ROOT_URLCONF = "config.urls"

TEMPLATES: list[dict] = []

WSGI_APPLICATION = "config.wsgi.application"

# HU-47.1: SQLite file-based. django.contrib.sessions.ready() hace un
# SELECT 1 al DB; con signed_cookies no se usa para storage pero igual
# necesita DB accesible. SQLite es lo más liviano (no requiere servicio).
DATABASES = {
    "default": {
        "ENGINE": "django.db.backends.sqlite3",
        "NAME": BASE_DIR / "db.sqlite3",
    }
}

# HU-47.1: sessions via signed cookies (default). No se usa DB para storage,
# solo para que el ready() no falle.
SESSION_ENGINE = "django.contrib.sessions.backends.signed_cookies"

DEFAULT_AUTO_FIELD = "django.db.models.BigAutoField"

USE_TZ = True
LANGUAGE_CODE = "es-ar"
TIME_ZONE = "UTC"

# HU-47.1: cookies seguras (HTTPS-only cuando DEBUG=False).
SESSION_COOKIE_SECURE = not DEBUG
SESSION_COOKIE_HTTPONLY = True
SESSION_COOKIE_SAMESITE = "Lax"
# 8 horas de sesión.
SESSION_COOKIE_AGE = 60 * 60 * 8