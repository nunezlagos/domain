"""Conftest para tests unitarios de la app chat.

Los tests del chat son UNITARIOS (con mocks) — no tocan la DB real. Por eso
no usan el runner de tests del proyecto (core.tests.runner) que requiere
Postgres real. En su lugar, configuramos un Django settings minimal solo
con lo necesario para que los imports funcionen (INSTALLED_APPS, DATABASES
dummy, etc).
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
            "chat",
        ],
        SECRET_KEY="test-secret",
        USE_TZ=True,
        DEFAULT_AUTO_FIELD="django.db.models.BigAutoField",
    )
    django.setup()