from django.apps import AppConfig


class AgentTemplatesConfig(AppConfig):
    default_auto_field = "django.db.models.BigAutoField"
    # name completo (ruta de import) bajo el paquete maintainers.
    name = "maintainers.agenttemplates"
    # app_label EXPLÍCITO: lo fijamos para dejar el contrato claro y a prueba de
    # refactors. Mantiene {% url 'agenttemplates:...' %}.
    label = "agenttemplates"
    verbose_name = "Mantenedor de Plantillas de Agentes"
