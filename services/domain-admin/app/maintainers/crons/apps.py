from django.apps import AppConfig


class CronsConfig(AppConfig):
    default_auto_field = "django.db.models.BigAutoField"

    name = "maintainers.crons"



    label = "crons"
    verbose_name = "Mantenedor de Crons (schedules)"
