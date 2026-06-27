"""Conftest para tests unitarios de la app feedback (HU-52.1).

Igual que la app chat: tests UNITARIOS con mocks, no tocan la DB real. Se
configura un Django settings minimal solo con lo necesario para que los
imports funcionen.
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
            "maintainers.feedback",
        ],
        SECRET_KEY="test-secret",
        USE_TZ=True,
        DEFAULT_AUTO_FIELD="django.db.models.BigAutoField",
    )
    django.setup()
