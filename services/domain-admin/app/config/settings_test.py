"""Settings para la suite de tests del admin.

Diferencias clave con producción:
- Los modelos de `users` (User/Role/UserRole) son managed=False en prod
  (las tablas las administra domain-mcp). Acá los flipeamos a managed=True
  para que el test-runner cree el schema en la DB de test efímera.
  Esto evita falsos positivos: los tests corren contra el ORM real + Postgres
  real (ArrayField, UUID, constraints), no contra mocks.
- Hasher rápido (no PBKDF2) para que crear users en tests no sea lento.
- Static storage plano (WhiteNoise manifest exige collectstatic, innecesario en test).
"""
from .settings import *  # noqa: F401,F403

# El flip managed=False→True lo hace el runner (después de cargar apps; no se
# pueden importar modelos acá porque settings se evalúa antes del app registry).
TEST_RUNNER = "users.tests.runner.ManagedModelTestRunner"

PASSWORD_HASHERS = ["django.contrib.auth.hashers.MD5PasswordHasher"]

# En prod la sesión es signed_cookies; el helper client.session de los tests no
# persiste bien con ese backend. Usamos sesión en DB solo en test para poder
# setear el flag `authenticated` de forma fiable.
SESSION_ENGINE = "django.contrib.sessions.backends.db"

STORAGES = {
    "default": {"BACKEND": "django.core.files.storage.FileSystemStorage"},
    "staticfiles": {"BACKEND": "django.contrib.staticfiles.storage.StaticFilesStorage"},
}
