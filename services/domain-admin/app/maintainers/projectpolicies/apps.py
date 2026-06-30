from django.apps import AppConfig


class ProjectPoliciesConfig(AppConfig):
    default_auto_field = "django.db.models.BigAutoField"
    name = "maintainers.projectpolicies"


    label = "projectpolicies"
    verbose_name = "Mantenedor de Reglas por proyecto"
