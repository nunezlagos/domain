"""HU-API.1: URL routing del mantenedor de API Keys.

Mounted at /api-keys/ en config/urls.py.
Segmentos literales (nuevo/, editar/, eliminar/, toggle/, signal/) para que
la derivación entity-agnostic de base.html funcione contra data-base-url.
"""
from django.urls import path

from . import views

# App namespace para {% url 'apikeys:list' %} etc.
app_name = "apikeys"

urlpatterns = [
    path("", views.apikey_list, name="list"),
    path("signal/", views.apikey_list_signal, name="signal"),
    path("nuevo/", views.apikey_create, name="create"),
    path("<uuid:apikey_id>/", views.apikey_detail, name="detail"),
    path("<uuid:apikey_id>/editar/", views.apikey_edit, name="edit"),
    path("<uuid:apikey_id>/eliminar/", views.apikey_delete, name="delete"),
    path("<uuid:apikey_id>/toggle/", views.apikey_toggle, name="toggle"),
]
