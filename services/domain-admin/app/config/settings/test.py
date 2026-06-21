"""Settings para la suite de tests del admin.

Diferencias clave con producción:
- Los modelos de `users` (User/Role/UserRole) son managed=False en prod
  (las tablas las administra domain-mcp). El test-runner los flipea a
  managed=True para crear el schema en la DB de test efímera.
  Esto evita falsos positivos: los tests corren contra el ORM real + Postgres
  real (ArrayField, UUID, constraints), no contra mocks.
- Hasher rápido (no PBKDF2) para que crear users en tests no sea lento.
- Static storage plano (WhiteNoise manifest exige collectstatic, innecesario en test).
- Sesión en DB (signed_cookies no persiste bien con client.session de los tests).

CÓMO CORRER LOS TESTS:
    # Opción A — apuntar directo al módulo de settings de test:
    DJANGO_SETTINGS_MODULE=config.settings.test python manage.py test
    # Opción B — usar el selector por entorno del paquete config.settings:
    DJANGO_ENV=test python manage.py test
"""
from .base import *  # noqa: F401,F403

# El flip managed=False→True lo hace el runner (después de cargar apps; no se
# pueden importar modelos acá porque settings se evalúa antes del app registry).
# Runner canónico del proyecto en core.tests.runner.
TEST_RUNNER = "core.tests.runner.ManagedModelTestRunner"

PASSWORD_HASHERS = ["django.contrib.auth.hashers.MD5PasswordHasher"]

# En prod la sesión es signed_cookies; el helper client.session de los tests no
# persiste bien con ese backend. Usamos sesión en DB solo en test para poder
# setear el flag `authenticated` de forma fiable.
SESSION_ENGINE = "django.contrib.sessions.backends.db"

STORAGES = {
    "default": {"BACKEND": "django.core.files.storage.FileSystemStorage"},
    "staticfiles": {"BACKEND": "django.contrib.staticfiles.storage.StaticFilesStorage"},
}
