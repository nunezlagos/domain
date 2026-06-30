"""HU-52.1: URL routing del feedback loop.

Se monta bajo /feedback/ desde config.urls.
"""
from django.urls import path

from . import views

app_name = "feedback"

urlpatterns = [
    path("", views.admin_list, name="list"),
    path("api/submit", views.submit, name="submit"),
]
