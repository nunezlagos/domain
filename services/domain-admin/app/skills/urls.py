"""URL routing del mantenedor de Skills.

Mounted at /skills/ en config/urls.py.

Orden importa: 'nuevo/' y 'signal/' antes que '<uuid:skill_id>/' para que
no sean capturados por el converter de uuid.

NO hay 'toggle/': la tabla skills no tiene columna `status`, así que no hay
estado alternable. SÍ hay 'eliminar/' (soft-delete vía deleted_at).
"""
from django.urls import path

from . import views

# App namespace para {% url 'skills:list' %} etc.
app_name = "skills"

urlpatterns = [
    path("", views.skill_list, name="list"),
    path("signal/", views.skill_list_signal, name="signal"),
    path("nuevo/", views.skill_create, name="create"),
    path("<uuid:skill_id>/", views.skill_detail, name="detail"),
    path("<uuid:skill_id>/editar/", views.skill_edit, name="edit"),
    path("<uuid:skill_id>/eliminar/", views.skill_delete, name="delete"),
]
