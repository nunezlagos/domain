from django.apps import AppConfig


class AgentsConfig(AppConfig):
    default_auto_field = "django.db.models.BigAutoField"

    name = "maintainers.agents"


    label = "agents"
    verbose_name = "Mantenedor de Agentes"
