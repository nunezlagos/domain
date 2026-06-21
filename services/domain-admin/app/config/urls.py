"""HU-47.2: URL routing del admin dashboard."""
from django.urls import path

from config import views

urlpatterns = [
    path("", views.login_view, name="home"),
    path("login/", views.login_view, name="login"),
    path("logout/", views.logout_view, name="logout"),
    path("dashboard/", views.dashboard, name="dashboard"),
    path("components/", views.components_demo, name="components"),
]