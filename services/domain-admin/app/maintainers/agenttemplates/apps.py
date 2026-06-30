from django.apps import AppConfig


class AgentTemplatesConfig(AppConfig):
    default_auto_field = "django.db.models.BigAutoField"

    name = "maintainers.agenttemplates"


    label = "agenttemplates"
    verbose_name = "Mantenedor de Plantillas de Agentes"
