"""URL routing del mantenedor de Proyectos.

Mounted at /proyectos/ en config/urls.py.
Segmentos literales (nuevo/ editar/ eliminar/ toggle/ signal/) requeridos por
la convención entity-agnostic de base.html (derivación de URLs desde data-base-url).
"""
from django.urls import path

from . import views

# App namespace para {% url 'projects:list' %} etc.
app_name = "projects"

urlpatterns = [
    path("", views.project_list, name="list"),
    path("signal/", views.project_list_signal, name="signal"),
    path("nuevo/", views.project_create, name="create"),
    path("<uuid:project_id>/", views.project_detail, name="detail"),
    path("<uuid:project_id>/editar/", views.project_edit, name="edit"),
    path("<uuid:project_id>/eliminar/", views.project_delete, name="delete"),
    path("<uuid:project_id>/toggle/", views.project_toggle, name="toggle"),
]
