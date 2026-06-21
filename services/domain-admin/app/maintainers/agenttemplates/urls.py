"""URL routing del mantenedor de Plantillas de Agentes.

Mounted at /plantillas-agentes/ en config/urls.py. Las 7 rutas estándar (list,
signal, create, detail, edit, delete, toggle) las arma
core.urls.maintainer_urlpatterns a partir de la instancia `views`. Este
mantenedor NO expone botón de toggle en la UI (status no se alterna; la baja es
HARD delete), pero la ruta queda cableada por el helper igual que en el resto;
simplemente no se usa.

app_name="agenttemplates" -> {% url 'agenttemplates:list' %}.
id_kwarg="template_id" debe coincidir con el id_kwarg del MaintainerViews.
"""
from core.urls import maintainer_urlpatterns

from . import views

app_name = "agenttemplates"

urlpatterns = maintainer_urlpatterns(views.views, id_kwarg="template_id")
