"""URL routing del mantenedor de Clientes (mandantes).

Mounted at /clientes/ en config/urls.py.

Orden importa: 'nuevo/' y 'signal/' antes que '<uuid:client_id>/' para que
no sean capturados por el converter de uuid.
"""
from django.urls import path

from . import views

# App namespace para {% url 'clients:list' %} etc.
app_name = "clients"

urlpatterns = [
    path("", views.client_list, name="list"),
    path("signal/", views.client_list_signal, name="signal"),
    path("nuevo/", views.client_create, name="create"),
    path("<uuid:client_id>/", views.client_detail, name="detail"),
    path("<uuid:client_id>/editar/", views.client_edit, name="edit"),
    path("<uuid:client_id>/eliminar/", views.client_delete, name="delete"),
    path("<uuid:client_id>/toggle/", views.client_toggle, name="toggle"),
]
