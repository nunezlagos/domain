"""HU-48.1: URL routing del admin dashboard."""
from django.urls import include, path

from config import views

urlpatterns = [
    path("", views.home_view, name="home"),
    path("login/", views.login_view, name="login"),
    path("logout/", views.logout_view, name="logout"),
    path("dashboard/", views.dashboard, name="dashboard"),
    path("componentes/", views.components_demo, name="components"),


    path("flujo-sdd/", views.sdd_flow, name="sdd_flow"),


    path("usuarios/", include("maintainers.users.urls")),


    path("proyectos/", include("maintainers.projects.urls")),
    path("api-keys/", include("maintainers.apikeys.urls")),


    path("agentes/", include("maintainers.agents.urls")),
    path("skills/", include("maintainers.skills.urls")),
    path("flows/", include("maintainers.flows.urls")),
    path("crons/", include("maintainers.crons.urls")),
    path("prompts/", include("maintainers.prompts.urls")),


    path("plantillas-agentes/", include("maintainers.agenttemplates.urls")),


    path("politicas-proyecto/", include("maintainers.projectpolicies.urls")),
    path("politicas-plataforma/", include("maintainers.platformpolicies.urls")),

    path("uso/", include("maintainers.usage.urls")),
]




handler400 = "config.views.bad_request"
handler403 = "config.views.permission_denied"
handler404 = "config.views.page_not_found"
handler500 = "config.views.server_error"