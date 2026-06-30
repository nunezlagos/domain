"""URL routing del mantenedor de API Keys (migrado a core).

Mounted at /api-keys/ en config/urls.py. Las 7 rutas estandar (list, signal,
create, detail, edit, delete, toggle) las arma core.urls.maintainer_urlpatterns
a partir de la instancia `views`. No hay rutas propias adicionales.

app_name="apikeys" -> {% url 'apikeys:list' %} sigue funcionando igual que antes.
id_kwarg="apikey_id" debe coincidir con el id_kwarg del MaintainerViews.
"""
from django.urls import path

from core.urls import maintainer_urlpatterns

from . import views

app_name = "apikeys"

urlpatterns = maintainer_urlpatterns(views.views, id_kwarg="apikey_id") + [
    path("export/", views.export_api_keys, name="export"),
]
