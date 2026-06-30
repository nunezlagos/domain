from django.apps import AppConfig


class UsersConfig(AppConfig):
    default_auto_field = "django.db.models.BigAutoField"

    name = "maintainers.users"



    label = "users"
    verbose_name = "Mantenedor de Usuarios"
