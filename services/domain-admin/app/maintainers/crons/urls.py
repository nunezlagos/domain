"""URL routing del mantenedor de Crons (schedules), migrado a core.

Mounted at /crons/ en config/urls.py. Las 7 rutas estandar (list, signal,
create, detail, edit, delete, toggle) las arma core.urls.maintainer_urlpatterns
a partir de la instancia `views`.

app_name="crons" -> {% url 'crons:list' %} sigue funcionando igual que antes.
id_kwarg="cron_id" debe coincidir con el id_kwarg del MaintainerViews.
"""
from django.urls import path

from core.urls import maintainer_urlpatterns

from . import views

app_name = "crons"

urlpatterns = maintainer_urlpatterns(views.views, id_kwarg="cron_id") + [
    path("export/", views.export_crons, name="export"),
]
