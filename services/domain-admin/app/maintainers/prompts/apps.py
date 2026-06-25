from django.apps import AppConfig


class PromptsConfig(AppConfig):
    default_auto_field = "django.db.models.BigAutoField"

    name = "maintainers.prompts"


    label = "prompts"
    verbose_name = "Mantenedor de Prompts"
