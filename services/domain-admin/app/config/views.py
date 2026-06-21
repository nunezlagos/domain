"""HU-47.2: views del admin dashboard.

Single-user hardcoded auth via env vars. CSRF + Messages activados.
"""
import os

from django.contrib import messages
from django.http import HttpResponseRedirect
from django.shortcuts import render
from django.urls import path
from django.views.decorators.csrf import csrf_protect


def _admin_creds() -> tuple[str, str]:
    return (
        os.environ.get("ADMIN_EMAIL", "admin@admin.com"),
        os.environ.get("ADMIN_PASSWORD", "q1w2e3r4"),
    )


def _is_authed(request) -> bool:
    return bool(request.session.get("authenticated"))


def _require_auth(request) -> HttpResponseRedirect | None:
    """Retorna redirect a /login/ si no está autenticado, None si OK."""
    if not _is_authed(request):
        return HttpResponseRedirect("/login/")
    return None


def login_view(request):
    # Si ya está autenticado, mandarlo al dashboard.
    if _is_authed(request):
        return HttpResponseRedirect("/dashboard/")

    if request.method == "POST":
        email = request.POST.get("email", "").strip()
        password = request.POST.get("password", "")
        admin_email, admin_password = _admin_creds()
        if email == admin_email and password == admin_password:
            # Anti session fixation: regenerar session key.
            request.session.cycle_key()
            request.session["authenticated"] = True
            request.session["email"] = admin_email
            messages.success(request, f"Bienvenido, {admin_email}")
            return HttpResponseRedirect("/dashboard/")
        messages.error(request, "Email o contraseña incorrectos.")

    return render(request, "login.html")


def home_view(request):
    """Raíz: redirige al panel o al login."""
    if _is_authed(request):
        return HttpResponseRedirect("/dashboard/")
    return HttpResponseRedirect("/login/")


@csrf_protect
def dashboard(request):
    redir = _require_auth(request)
    if redir:
        return redir
    return render(request, "dashboard.html")


@csrf_protect
def components_demo(request):
    redir = _require_auth(request)
    if redir:
        return redir
    return render(request, "components_demo.html")


def logout_view(request):
    request.session.flush()
    messages.info(request, "Sesión cerrada.")
    return HttpResponseRedirect("/login/")


urlpatterns = [
    path("", login_view, name="home"),
    path("login/", login_view, name="login"),
    path("dashboard/", dashboard, name="dashboard"),
    path("components/", components_demo, name="components"),
    path("logout/", logout_view, name="logout"),
]