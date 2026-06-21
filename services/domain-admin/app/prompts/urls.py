"""URL routing del mantenedor de Prompts.

Mounted at /prompts/ en config/urls.py.

Orden importa: 'nuevo/' y 'signal/' antes que '<uuid:prompt_id>/' para que
no sean capturados por el converter de uuid.
"""
from django.urls import path

from . import views

# App namespace para {% url 'prompts:list' %} etc.
app_name = "prompts"

urlpatterns = [
    path("", views.prompt_list, name="list"),
    path("signal/", views.prompt_list_signal, name="signal"),
    path("nuevo/", views.prompt_create, name="create"),
    path("<uuid:prompt_id>/", views.prompt_detail, name="detail"),
    path("<uuid:prompt_id>/editar/", views.prompt_edit, name="edit"),
    path("<uuid:prompt_id>/eliminar/", views.prompt_delete, name="delete"),
    path("<uuid:prompt_id>/toggle/", views.prompt_toggle, name="toggle"),
]
