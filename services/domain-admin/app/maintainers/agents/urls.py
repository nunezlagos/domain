"""URL routing del mantenedor de Agentes (migrado a core).

Mounted at /agentes/ en config/urls.py. Las rutas estándar (list, signal,
create, detail, edit, delete, toggle) las arma core.urls.maintainer_urlpatterns
a partir de la instancia `views`. No hay rutas propias extra.

app_name="agents" -> {% url 'agents:list' %} sigue funcionando igual que antes.
id_kwarg="agent_id" debe coincidir con el id_kwarg del MaintainerViews.

NOTA: el helper cablea también 'toggle/' (parte del CRUD estándar). agents NO
expone toggle en su UI (su único cambio de estado es el soft-delete), así que la
ruta queda disponible pero ningún template la enlaza.
"""
from django.urls import path  # noqa: F401 (disponible para rutas extra futuras)

from core.urls import maintainer_urlpatterns

from . import views

app_name = "agents"

urlpatterns = maintainer_urlpatterns(views.views, id_kwarg="agent_id")
