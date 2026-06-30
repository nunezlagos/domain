"""URL routing del mantenedor de Flows (migrado a core).

Mounted at /flows/ en config/urls.py. Las 7 rutas estandar (list, signal,
create, detail, edit, delete, toggle) las arma core.urls.maintainer_urlpatterns
a partir de la instancia `views`.

app_name="flows" -> {% url 'flows:list' %} sigue funcionando igual que antes.
id_kwarg="flow_id" debe coincidir con el id_kwarg del MaintainerViews.

Los segmentos en español (nuevo/editar/eliminar/toggle) que arma core.urls son
OBLIGATORIOS: maintainer.js deriva las URLs de accion desde data-base-url +
estos segmentos. core.urls los mantiene identicos a los del app original.
"""
from django.urls import path

from core.urls import maintainer_urlpatterns

from . import views

app_name = "flows"

urlpatterns = maintainer_urlpatterns(views.views, id_kwarg="flow_id") + [
    path("export/", views.export_flows, name="export"),
]
