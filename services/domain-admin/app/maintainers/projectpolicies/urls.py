"""URL routing del mantenedor de Politicas por proyecto.

Mounted at /politicas-proyecto/ en config/urls.py. Las 7 rutas estandar las arma
core.urls.maintainer_urlpatterns. id_kwarg="policy_id".
"""
from django.urls import path
from django.views.decorators.http import require_http_methods

from core.urls import maintainer_urlpatterns

from . import views

app_name = "projectpolicies"

urlpatterns = maintainer_urlpatterns(views.views, id_kwarg="policy_id") + [
    path("<uuid:policy_id>/aprobar/", require_http_methods(["POST"])(views.approve_policy), name="approve"),
    path("<uuid:policy_id>/rechazar/", require_http_methods(["POST"])(views.reject_policy), name="reject"),
]
