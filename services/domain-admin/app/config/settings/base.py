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
    "maintainers.projectpolicies",  # mantenedor de politicas por proyecto
    "maintainers.platformpolicies",  # mantenedor de politicas de plataforma
    "maintainers.usage",            # dashboard de uso (captured_prompts KPIs)
    "maintainers.mcpuptime",        # monitoreo uptime/health del server domain-mcp
    "maintainers.feedback",         # HU-52.1: feedback loop (👍/👎) sobre respuestas del chat
    "maintainers.skillsuggestions",  # HU-52.3: LLM-as-judge (human-in-the-loop) sobre skills
    "chat",                          # HU-49.2: chat IA estilo NotebookLM
]

MIDDLEWARE = [
    "django.contrib.sessions.middleware.SessionMiddleware",
    "django.contrib.messages.middleware.MessageMiddleware",
    "django.middleware.csrf.CsrfViewMiddleware",   # HU-47.2: CSRF real con templates
]

ROOT_URLCONF = "config.urls"


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
                # HU-52.3: badge de pendientes en el sidebar (todas las paginas).
                "maintainers.skillsuggestions.context_processors.pending_badge",
            ],
        },
    },
]

WSGI_APPLICATION = "config.wsgi.application"






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






FIELD_ENC_KEY = os.environ.get("DOMAIN_FIELD_ENC_KEY", "")


# Google OAuth para login con Gmail.
# Client ID del proyecto en Google Cloud Console (credencial OAuth Web).
# Si está vacío, el botón "Login con Gmail" no se muestra.
GOOGLE_CLIENT_ID = os.environ.get("GOOGLE_CLIENT_ID", "")


STATIC_URL = "/static/"
STATICFILES_DIRS = [BASE_DIR / "static"]
STATIC_ROOT = BASE_DIR / "staticfiles"  # collectstatic destination


MIDDLEWARE.insert(0, "whitenoise.middleware.WhiteNoiseMiddleware")
STATICFILES_STORAGE = "whitenoise.storage.CompressedManifestStaticFilesStorage"


SESSION_COOKIE_SECURE = os.environ.get("SESSION_COOKIE_SECURE", "0") == "1"
SESSION_COOKIE_HTTPONLY = True
SESSION_COOKIE_SAMESITE = "Lax"
SESSION_COOKIE_AGE = 60 * 60 * 8  # 8 horas


CSRF_TRUSTED_ORIGINS = [
    "http://13.140.183.236",
    "http://localhost",
]

DEFAULT_AUTO_FIELD = "django.db.models.BigAutoField"

# UUID de la org por defecto (single-org). Se usa para SET LOCAL app.current_org_id
# en consultas a tablas con FORCE RLS (ej. captured_prompts). En prod debe
# coincidir con el org_id real del deployment.
DEFAULT_ORG_ID = os.environ.get("DEFAULT_ORG_ID", "00000000-0000-0000-0000-000000000001")


# Monitoreo de uptime/health del server domain-mcp.
# El command `poll_mcp_health` hace GET aca y registra en mcp_health_checks.
# Prioridad: DOMAIN_MCP_HEALTH_URL > DOMAIN_BASE_URL + "/health" > default interno.
DOMAIN_MCP_HEALTH_URL = os.environ.get("DOMAIN_MCP_HEALTH_URL", "")
DOMAIN_BASE_URL = os.environ.get("DOMAIN_BASE_URL", "")


# HU-52.3: cliente server-to-server al REST del domain-mcp para transicionar
# skill_suggestions (approve/reject/apply). El Bearer NUNCA llega al browser.
# - DOMAIN_API_BASE_URL: base del REST (default DOMAIN_BASE_URL, luego interno).
# - DOMAIN_API_TOKEN: Bearer de una API key con permiso. Si falta, las
#   transiciones devuelven error claro (no se auto-aplica nada; regla dura 6/7).
DOMAIN_API_BASE_URL = os.environ.get("DOMAIN_API_BASE_URL", "")
DOMAIN_API_TOKEN = os.environ.get("DOMAIN_API_TOKEN", "")





SILENCED_SYSTEM_CHECKS = ["fields.W342"]

USE_TZ = True
LANGUAGE_CODE = "es-ar"
TIME_ZONE = "UTC"
