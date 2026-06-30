from django.apps import AppConfig


class UsageConfig(AppConfig):
    default_auto_field = "django.db.models.BigAutoField"
    name = "maintainers.usage"
    label = "usage"
    verbose_name = "Dashboard de Uso"
