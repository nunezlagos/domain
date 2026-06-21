from django.apps import AppConfig


class ApikeysConfig(AppConfig):
    default_auto_field = "django.db.models.BigAutoField"
    # name completo (ruta de import) bajo el paquete maintainers.
    name = "maintainers.apikeys"
    # app_label EXPLÍCITO: mantiene {% url 'apikeys:...' %} y el guard de
    # schema drift (core/tests/test_schema_drift.py espera label "apikeys").
    label = "apikeys"
    verbose_name = "Mantenedor de API Keys"
