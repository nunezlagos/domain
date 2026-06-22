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
    # Opcion A — apuntar directo al modulo de settings de test:
    DJANGO_SETTINGS_MODULE=config.settings.test python manage.py test
    # Opcion B — usar el selector por entorno del paquete config.settings:
    DJANGO_ENV=test python manage.py test
"""
from .base import *  # noqa: F401,F403

# El flip managed=False→True lo hace el runner (despues de cargar apps; no se
# pueden importar modelos aqui porque settings se evalua antes del app registry).
# Runner canonico del proyecto en core.tests.runner.
TEST_RUNNER = "core.tests.runner.ManagedModelTestRunner"

PASSWORD_HASHERS = ["django.contrib.auth.hashers.MD5PasswordHasher"]

# Passphrase de cifrado at-rest para los tests (en prod viene del env
# DOMAIN_FIELD_ENC_KEY). Sin esto, create_api_key/get_api_key_plaintext fallan.
FIELD_ENC_KEY = "test-field-enc-key"

# En prod la sesion es signed_cookies; el helper client.session de los tests no
# persiste bien con ese backend. Usamos sesion en DB solo en test para poder
# setear el flag `authenticated` de forma fiable.
SESSION_ENGINE = "django.contrib.sessions.backends.db"

STORAGES = {
    "default": {"BACKEND": "django.core.files.storage.FileSystemStorage"},
    "staticfiles": {"BACKEND": "django.contrib.staticfiles.storage.StaticFilesStorage"},
}
