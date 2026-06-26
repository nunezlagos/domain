"""URL routing del mantenedor de Politicas de plataforma.

Mounted at /politicas-plataforma/ en config/urls.py. Las 7 rutas estandar las
arma core.urls.maintainer_urlpatterns. id_kwarg="policy_id".
"""
from core.urls import maintainer_urlpatterns

from . import views

app_name = "platformpolicies"

urlpatterns = maintainer_urlpatterns(views.views, id_kwarg="policy_id")
