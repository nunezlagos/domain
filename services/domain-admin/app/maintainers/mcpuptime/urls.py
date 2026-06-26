from django.urls import path

from . import views

app_name = "mcpuptime"

urlpatterns = [
    path("", views.dashboard, name="dashboard"),
]
