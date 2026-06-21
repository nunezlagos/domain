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
    path("usuarios/", include("maintainers.users.urls")),

    # Mantenedores de proyectos y API keys
    path("proyectos/", include("projects.urls")),
    path("api-keys/", include("apikeys.urls")),

    # Mantenedores de agentes, skills, flows, crons y prompts
    path("agentes/", include("agents.urls")),
    path("skills/", include("skills.urls")),
    path("flows/", include("flows.urls")),
    path("crons/", include("crons.urls")),
    path("prompts/", include("prompts.urls")),
]

# Handlers de error a nivel módulo. Django los resuelve por nombre de variable
# (handler400/403/404/500) apuntando a vistas que renderizan
# templates/errors/{code}.html. El handler500 solo se usa con DEBUG=False.
handler400 = "config.views.bad_request"
handler403 = "config.views.permission_denied"
handler404 = "config.views.page_not_found"
handler500 = "config.views.server_error"