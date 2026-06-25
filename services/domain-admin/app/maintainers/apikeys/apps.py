from django.apps import AppConfig


class ApikeysConfig(AppConfig):
    default_auto_field = "django.db.models.BigAutoField"

    name = "maintainers.apikeys"


    label = "apikeys"
    verbose_name = "Mantenedor de API Keys"
