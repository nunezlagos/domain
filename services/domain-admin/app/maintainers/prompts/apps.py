from django.apps import AppConfig


class PromptsConfig(AppConfig):
    default_auto_field = "django.db.models.BigAutoField"
    # name completo (ruta de import) bajo el paquete maintainers.
    name = "maintainers.prompts"
    # app_label EXPLÍCITO: mantiene {% url 'prompts:...' %} y el guard de
    # schema drift (core/tests/test_schema_drift.py espera label "prompts").
    label = "prompts"
    verbose_name = "Mantenedor de Prompts"
