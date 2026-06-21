"""URL routing del mantenedor de Crons (schedules).

Mounted at /crons/ en config/urls.py.

Orden importa: 'nuevo/' y 'signal/' antes que '<uuid:cron_id>/' para que
no sean capturados por el converter de uuid.
"""
from django.urls import path

from . import views

# App namespace para {% url 'crons:list' %} etc.
app_name = "crons"

urlpatterns = [
    path("", views.cron_list, name="list"),
    path("signal/", views.cron_list_signal, name="signal"),
    path("nuevo/", views.cron_create, name="create"),
    path("<uuid:cron_id>/", views.cron_detail, name="detail"),
    path("<uuid:cron_id>/editar/", views.cron_edit, name="edit"),
    path("<uuid:cron_id>/eliminar/", views.cron_delete, name="delete"),
    path("<uuid:cron_id>/toggle/", views.cron_toggle, name="toggle"),
]
