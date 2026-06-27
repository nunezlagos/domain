"""HU-52.3: URL routing de la app skillsuggestions.

Se monta bajo /skill-suggestions/ desde config.urls.
"""
from django.urls import path

from . import views

app_name = "skillsuggestions"

urlpatterns = [
    path("", views.admin_list, name="list"),
    path("<uuid:suggestion_id>/", views.detail, name="detail"),
    path("<uuid:suggestion_id>/approve", views.approve_view, name="approve"),
    path("<uuid:suggestion_id>/reject", views.reject_view, name="reject"),
    path("<uuid:suggestion_id>/apply", views.apply_view, name="apply"),
]
