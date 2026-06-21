"""URL routing del mantenedor de Proyectos (migrado a core).

Mounted at /proyectos/ en config/urls.py. Las 7 rutas estándar (list, signal,
create, detail, edit, delete, toggle) las arma core.urls.maintainer_urlpatterns
a partir de la instancia `views`.

app_name="projects" -> {% url 'projects:list' %} sigue funcionando igual que antes.
id_kwarg="project_id" debe coincidir con el id_kwarg del MaintainerViews.
"""
from core.urls import maintainer_urlpatterns

from . import views

app_name = "projects"

urlpatterns = maintainer_urlpatterns(views.views, id_kwarg="project_id")
