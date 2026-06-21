"""URL routing del mantenedor de Agentes.

Mounted at /agentes/ en config/urls.py.

Orden importa: 'nuevo/' y 'signal/' antes que '<uuid:agent_id>/' para que
no sean capturados por el converter de uuid.

NO hay segmento 'toggle/': la tabla agents no tiene columna status, así que
no existe estado alternable. Sí hay 'eliminar/' (soft-delete vía deleted_at).
"""
from django.urls import path

from . import views

# App namespace para {% url 'agents:list' %} etc.
app_name = "agents"

urlpatterns = [
    path("", views.agent_list, name="list"),
    path("signal/", views.agent_list_signal, name="signal"),
    path("nuevo/", views.agent_create, name="create"),
    path("<uuid:agent_id>/", views.agent_detail, name="detail"),
    path("<uuid:agent_id>/editar/", views.agent_edit, name="edit"),
    path("<uuid:agent_id>/eliminar/", views.agent_delete, name="delete"),
]
