"""Settings comunes a todos los entornos.

HISTORIAL:
- HU-45.1: placeholder estatico
- HU-47.1: login simple single-user
- HU-47.2: design system (sidebar, navbar, footer, components)
- HU-48.x: ORM (Django models managed=False contra Postgres de domain-mcp)

Este modulo NO debe importarse directo como DJANGO_SETTINGS_MODULE.
Usa el paquete `config.settings` (selecciona prod/test via DJANGO_ENV).
"""
import os
from pathlib import Path

# base.py vive en app/config/settings/ -> parent.parent.parent == app/
BASE_DIR = Path(__file__).resolve().parent.parent.parent

SECRET_KEY = "hu-47.2-rotate-me-before-prod-3f8a9b2c1d4e5f6a"
DEBUG = os.environ.get("DJANGO_DEBUG", "0") == "1"
ALLOWED_HOSTS = ["*"]

INSTALLED_APPS = [
    "django.contrib.sessions",
    "django.contrib.messages",      # HU-47.2: para flash messages (alerts)
    "django.contrib.staticfiles",
    "core",                         # paquete transversal (base models/services/views)
    "maintainers.users",            # HU-48: mantenedor de usuarios (app_label "users")
    "maintainers.projects",         # mantenedor de proyectos
    "maintainers.apikeys",          # mantenedor de API keys
    "maintainers.agents",           # mantenedor de agentes
    "maintainers.skills",           # mantenedor de skills
    "maintainers.flows",            # mantenedor de flows
    "maintainers.crons",            # mantenedor de crons (schedules)
    "maintainers.prompts",          # mantenedor de prompts
    "maintainers.agenttemplates",   # mantenedor de plantillas de agentes
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

# HU-48.1: las tablas reales (users, roles, user_roles, etc.) viven en
# Postgres (donde corre domain-mcp). Django NO las migra (managed=False)
# pero querya contra la DB configurada acá.
#
# Credenciales tomadas del env (.env) — mismas que usa domain-mcp.
DATABASES = {
    "default": {
        "ENGINE": "django.db.backends.postgresql",
        "NAME": os.environ.get("POSTGRES_DB", "domain"),
        "USER": os.environ.get("DB_APP_USER", "app_user"),
        "PASSWORD": os.environ.get("APP_USER_PASSWORD", ""),
        "HOST": os.environ.get("DB_HOST", "postgres"),
        "PORT": os.environ.get("DB_PORT", "5432"),
        "OPTIONS": {
            "sslmode": "disable",
        },
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

# W342: UserRole usa un FK con primary_key=True (db_column user_id) como
# workaround a la PK COMPUESTA de user_roles, que Django 5.1 no soporta nativo.
# Django sugiere OneToOneField, pero sería semánticamente incorrecto (un user
# tiene muchos roles). El warning es esperado y no aplica acá.
SILENCED_SYSTEM_CHECKS = ["fields.W342"]

USE_TZ = True
LANGUAGE_CODE = "es-ar"
TIME_ZONE = "UTC"
