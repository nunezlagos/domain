from django.apps import AppConfig


class AgentsConfig(AppConfig):
    default_auto_field = "django.db.models.BigAutoField"
    # name completo (ruta de import) bajo el paquete maintainers.
    name = "maintainers.agents"
    # app_label EXPLICITO: mantiene {% url 'agents:...' %} y el guard de
    # schema drift (MAINTAINER_APPS espera el label "agents").
    label = "agents"
    verbose_name = "Mantenedor de Agentes"
