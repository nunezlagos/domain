"""HU-48.1: URL routing del admin dashboard."""
from django.urls import include, path

from config import views

urlpatterns = [
    path("", views.home_view, name="home"),
    path("login/", views.login_view, name="login"),
    path("logout/", views.logout_view, name="logout"),
    path("dashboard/", views.dashboard, name="dashboard"),
    path("componentes/", views.components_demo, name="components"),

    # Mantenedor de usuarios (HU-48)
    path("usuarios/", include("users.urls")),

    # Mantenedores de clientes, proyectos y API keys
    path("clientes/", include("clients.urls")),
    path("proyectos/", include("projects.urls")),
    path("api-keys/", include("apikeys.urls")),

    # Mantenedores de agentes, skills, flows, crons y prompts
    path("agentes/", include("agents.urls")),
    path("skills/", include("skills.urls")),
    path("flows/", include("flows.urls")),
    path("crons/", include("crons.urls")),
    path("prompts/", include("prompts.urls")),
]