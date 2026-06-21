"""HU-47.2: Django settings con frontend design system.

HISTORIAL:
- HU-45.1: placeholder estatico
- HU-47.1: login simple single-user
- HU-47.2: design system (sidebar, navbar, footer, components)

PRÓXIMOS:
- HU-45.2: ORM (Django models) + User multi-user
"""
import os
from pathlib import Path

BASE_DIR = Path(__file__).resolve().parent.parent

SECRET_KEY = "hu-47.2-rotate-me-before-prod-3f8a9b2c1d4e5f6a"
DEBUG = os.environ.get("DJANGO_DEBUG", "0") == "1"
ALLOWED_HOSTS = ["*"]

INSTALLED_APPS = [
    "django.contrib.sessions",
    "django.contrib.messages",      # HU-47.2: para flash messages (alerts)
    "django.contrib.staticfiles",
]

MIDDLEWARE = [
    "django.contrib.sessions.middleware.SessionMiddleware",
    "django.contrib.messages.middleware.MessageMiddleware",
    "django.middleware.csrf.CsrfViewMiddleware",   # HU-47.2: CSRF real con templates
]

ROOT_URLCONF = "config.urls"

# HU-47.2: Django templates con partials y herencia.
TEMPLATES = [
    {
        "BACKEND": "django.template.backends.django.DjangoTemplates",
        "DIRS": [BASE_DIR / "templates"],
        "APP_DIRS": True,
        "OPTIONS": {
            "context_processors": [
                "django.template.context_processors.debug",
                "django.template.context_processors.request",
                "django.contrib.messages.context_processors.messages",
            ],
        },
    },
]

WSGI_APPLICATION = "config.wsgi.application"

# HU-47.1: SQLite file-based (django.contrib.sessions.ready() requiere DB).
# HU-47.2: signed_cookies sessions (no usa DB en runtime).
DATABASES = {
    "default": {
        "ENGINE": "django.db.backends.sqlite3",
        "NAME": BASE_DIR / "db.sqlite3",
    }
}
SESSION_ENGINE = "django.contrib.sessions.backends.signed_cookies"

# HU-47.2: static files con WhiteNoise (servido por gunicorn directo).
STATIC_URL = "/static/"
STATICFILES_DIRS = [BASE_DIR / "static"]
STATIC_ROOT = BASE_DIR / "staticfiles"  # collectstatic destination

# WHITENOISE settings: servir archivos estáticos sin nginx.
MIDDLEWARE.insert(0, "whitenoise.middleware.WhiteNoiseMiddleware")
STATICFILES_STORAGE = "whitenoise.storage.CompressedManifestStaticFilesStorage"

# Cookie config (HU-47.1): default HTTP, opt-in HTTPS via env.
SESSION_COOKIE_SECURE = os.environ.get("SESSION_COOKIE_SECURE", "0") == "1"
SESSION_COOKIE_HTTPONLY = True
SESSION_COOKIE_SAMESITE = "Lax"
SESSION_COOKIE_AGE = 60 * 60 * 8  # 8 horas

# CSRF: confiar en el host del VPS (Caddy proxy).
CSRF_TRUSTED_ORIGINS = [
    "http://13.140.183.236",
    "http://localhost",
]

DEFAULT_AUTO_FIELD = "django.db.models.BigAutoField"

USE_TZ = True
LANGUAGE_CODE = "es-ar"
TIME_ZONE = "UTC"