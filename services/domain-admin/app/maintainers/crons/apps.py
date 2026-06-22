from django.apps import AppConfig


class CronsConfig(AppConfig):
    default_auto_field = "django.db.models.BigAutoField"
    # name completo (ruta de import) bajo el paquete maintainers.
    name = "maintainers.crons"
    # app_label EXPLICITO: sin esto Django tomaria el ultimo segmento "crons"
    # igual, pero lo fijamos para dejar el contrato claro y a prueba de
    # refactors. Mantiene {% url 'crons:...' %} y el guard de schema drift.
    label = "crons"
    verbose_name = "Mantenedor de Crons (schedules)"
