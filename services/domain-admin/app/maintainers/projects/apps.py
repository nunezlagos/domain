from django.apps import AppConfig


class ProjectsConfig(AppConfig):
    default_auto_field = "django.db.models.BigAutoField"
    # name completo (ruta de import) bajo el paquete maintainers.
    name = "maintainers.projects"
    # app_label EXPLÍCITO: mantiene {% url 'projects:...' %} y el guard de
    # schema drift (core/tests/test_schema_drift.py espera label "projects").
    label = "projects"
    verbose_name = "Mantenedor de Proyectos"
