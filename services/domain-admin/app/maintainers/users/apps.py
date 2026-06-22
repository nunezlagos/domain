from django.apps import AppConfig


class UsersConfig(AppConfig):
    default_auto_field = "django.db.models.BigAutoField"
    # name completo (ruta de import) bajo el paquete maintainers.
    name = "maintainers.users"
    # app_label EXPLICITO: sin esto Django tomaria el ultimo segmento "users"
    # igual, pero lo fijamos para dejar el contrato claro y a prueba de
    # refactors. Mantiene {% url 'users:...' %} y el guard de schema drift.
    label = "users"
    verbose_name = "Mantenedor de Usuarios"
