"""HU-45.1: Django settings minimal para el placeholder admin.

Cero DB, cero apps que no sean las built-in de Django.
HU-45.2 va a sumar DATABASES + read replica.
HU-45.3 va a sumar sessions + auth contra el MCP.
"""
from pathlib import Path

BASE_DIR = Path(__file__).resolve().parent.parent

SECRET_KEY = "hu-45.1-placeholder-not-for-prod-replace-in-hu-45.3"
DEBUG = False
ALLOWED_HOSTS = ["*"]

INSTALLED_APPS = [
    "django.contrib.contenttypes",
    "django.contrib.auth",
    "django.contrib.staticfiles",
]

MIDDLEWARE: list[str] = []

ROOT_URLCONF = "config.urls"

TEMPLATES: list[dict] = []

WSGI_APPLICATION = "config.wsgi.application"

# HU-45.1: sin DB. Django igual requiere algo; dejar dict vacío.
DATABASES: dict = {}

DEFAULT_AUTO_FIELD = "django.db.models.BigAutoField"

USE_TZ = True
LANGUAGE_CODE = "es-ar"
TIME_ZONE = "UTC"