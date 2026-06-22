from django.apps import AppConfig


class FlowsConfig(AppConfig):
    default_auto_field = "django.db.models.BigAutoField"
    # name completo (ruta de import) bajo el paquete maintainers.
    name = "maintainers.flows"
    # app_label EXPLICITO: sin esto Django tomaria el ultimo segmento "flows"
    # igual, pero lo fijamos para dejar el contrato claro y a prueba de
    # refactors. Mantiene {% url 'flows:...' %} y el guard de schema drift.
    label = "flows"
    verbose_name = "Mantenedor de Flows"
