from django.apps import AppConfig


class SkillsConfig(AppConfig):
    default_auto_field = "django.db.models.BigAutoField"
    # name completo (ruta de import) bajo el paquete maintainers.
    name = "maintainers.skills"
    # app_label EXPLÍCITO: sin esto Django tomaría el último segmento "skills"
    # igual, pero lo fijamos para dejar el contrato claro y a prueba de
    # refactors. Mantiene {% url 'skills:...' %} y el guard de schema drift.
    label = "skills"
    verbose_name = "Mantenedor de Skills"
