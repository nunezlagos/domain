"""URL routing del mantenedor de Reglas por proyecto.

Mounted at /reglas-proyecto/ en config/urls.py. Las 7 rutas estándar las arma
core.urls.maintainer_urlpatterns. id_kwarg="policy_id".
"""
from core.urls import maintainer_urlpatterns

from . import views

app_name = "projectpolicies"

urlpatterns = maintainer_urlpatterns(views.views, id_kwarg="policy_id")
