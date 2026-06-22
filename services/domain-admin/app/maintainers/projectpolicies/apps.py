from django.apps import AppConfig


class ProjectPoliciesConfig(AppConfig):
    default_auto_field = "django.db.models.BigAutoField"
    name = "maintainers.projectpolicies"
    # app_label explicito: mantiene {% url 'projectpolicies:...' %} y el guard
    # de schema drift (core/tests/test_schema_drift.py espera este label).
    label = "projectpolicies"
    verbose_name = "Mantenedor de Reglas por proyecto"
