"""Settings para la suite de tests del admin.

Diferencias clave con produccion:
- Los modelos de `users` (User/Role/UserRole) son managed=False en prod
  (las tablas las administra domain-mcp). El test-runner los flipea a
  managed=True para crear el schema en la DB de test efimera.
  Esto evita falsos positivos: los tests corren contra el ORM real + Postgres
  real (ArrayField, UUID, constraints), no contra mocks.
- Hasher rapido (no PBKDF2) para que crear users en tests no sea lento.
- Static storage plano (WhiteNoise manifest exige collectstatic, innecesario en test).
- Sesion en DB (signed_cookies no persiste bien con client.session de los tests).

COMO CORRER LOS TESTS:

    DJANGO_SETTINGS_MODULE=config.settings.test python manage.py test

    DJANGO_ENV=test python manage.py test
"""
from .base import *  # noqa: F401,F403




TEST_RUNNER = "core.tests.runner.ManagedModelTestRunner"

PASSWORD_HASHERS = ["django.contrib.auth.hashers.MD5PasswordHasher"]



FIELD_ENC_KEY = "test-field-enc-key"




SESSION_ENGINE = "django.contrib.sessions.backends.db"

STORAGES = {
    "default": {"BACKEND": "django.core.files.storage.FileSystemStorage"},
    "staticfiles": {"BACKEND": "django.contrib.staticfiles.storage.StaticFilesStorage"},
}
