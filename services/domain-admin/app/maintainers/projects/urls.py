"""URL routing del mantenedor de Proyectos (migrado a core).

Mounted at /proyectos/ en config/urls.py. Las 7 rutas estandar (list, signal,
create, detail, edit, delete, toggle) las arma core.urls.maintainer_urlpatterns
a partir de la instancia `views`.

app_name="projects" -> {% url 'projects:list' %} sigue funcionando igual que antes.
id_kwarg="project_id" debe coincidir con el id_kwarg del MaintainerViews.
"""
from django.urls import path

from core.urls import maintainer_urlpatterns

from . import views

app_name = "projects"

urlpatterns = [


    path("<uuid:project_id>/skills/pane/", views.skills_pane, name="skills_pane"),
    path("<uuid:project_id>/skills/toggle/", views.toggle_skill, name="toggle_skill"),
    path("<uuid:project_id>/reglas/pane/", views.rules_pane, name="rules_pane"),
    path("<uuid:project_id>/reglas/toggle/", views.toggle_rule, name="toggle_rule"),

    path("export/", views.export_projects, name="export"),
] + maintainer_urlpatterns(views.views, id_kwarg="project_id")
