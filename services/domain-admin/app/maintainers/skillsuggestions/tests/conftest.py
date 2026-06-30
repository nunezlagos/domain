"""Conftest para tests unitarios de la app skillsuggestions (HU-52.3).

Tests UNITARIOS con mocks: no tocan la DB real ni el domain-mcp. Se configura
un Django settings minimal solo para que los imports funcionen.
"""
import django
from django.conf import settings


def pytest_configure(config):
    if settings.configured:
        return
    settings.configure(
        DEBUG=False,
        DATABASES={
            "default": {
                "ENGINE": "django.db.backends.sqlite3",
                "NAME": ":memory:",
            }
        },
        INSTALLED_APPS=[
            "django.contrib.contenttypes",
            "django.contrib.auth",
            "core",
            "maintainers.skillsuggestions",
        ],
        SECRET_KEY="test-secret",
        USE_TZ=True,
        DEFAULT_AUTO_FIELD="django.db.models.BigAutoField",
        DOMAIN_API_BASE_URL="http://domain-mcp:8080",
        DOMAIN_API_TOKEN="test-token",
    )
    django.setup()
