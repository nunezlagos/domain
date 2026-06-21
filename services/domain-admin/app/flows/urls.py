"""URL routing del mantenedor de Flows.

Mounted at /flows/ en config/urls.py.

Orden importa: 'signal/' y 'nuevo/' antes que '<uuid:flow_id>/' para que
no sean capturados por el converter de uuid.

Los segmentos en español (nuevo/editar/eliminar/toggle) son OBLIGATORIOS:
el handler global de modales en base.html deriva las URLs de acción desde
data-base-url + estos segmentos. Cambiarlos rompe el handler.
"""
from django.urls import path

from . import views

# App namespace para {% url 'flows:list' %} etc.
app_name = "flows"

urlpatterns = [
    path("", views.flow_list, name="list"),
    path("signal/", views.flow_list_signal, name="signal"),
    path("nuevo/", views.flow_create, name="create"),
    path("<uuid:flow_id>/", views.flow_detail, name="detail"),
    path("<uuid:flow_id>/editar/", views.flow_edit, name="edit"),
    path("<uuid:flow_id>/eliminar/", views.flow_delete, name="delete"),
    path("<uuid:flow_id>/toggle/", views.flow_toggle, name="toggle"),
]
