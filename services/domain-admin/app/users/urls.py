"""HU-48.1: URL routing del mantenedor de usuarios.

Mounted at /usuarios/ en config/urls.py.
"""
from django.urls import path

from . import views

# App namespace para {% url 'users:list' %} etc.
app_name = "users"

urlpatterns = [
    path("", views.user_list, name="list"),
    path("nuevo/", views.user_create, name="create"),
    path("<uuid:user_id>/", views.user_detail, name="detail"),
    path("<uuid:user_id>/editar/", views.user_edit, name="edit"),
    path("<uuid:user_id>/eliminar/", views.user_delete, name="delete"),
    path("<uuid:user_id>/toggle/", views.user_toggle, name="toggle"),
    path("<uuid:user_id>/roles/asignar/", views.role_assign, name="role_assign"),
    path("<uuid:user_id>/roles/<uuid:role_id>/revocar/", views.role_revoke, name="role_revoke"),
]