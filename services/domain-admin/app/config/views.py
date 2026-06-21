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


# --- Flujo SDD (vista general, no-CRUD) --------------------------------------
# Orden canónico de las 10 fases del pipeline SDD + el grupo (concern) de cada
# una para colorear el diagrama. El grupo ata visualmente la fase a la taxonomía
# sdd_/tdd_: spec (explore->tasks), exec (apply), tdd (verify/judge),
# close (archive/onboard).
_SDD_PHASES = [
    ("sdd-explore", "Explore", "spec", "Mapea el contexto y el código existente.", "search"),
    ("sdd-spec", "Spec", "spec", "Define el contrato y los criterios de aceptación.", "doc"),
    ("sdd-propose", "Propose", "spec", "Propone enfoques con sus tradeoffs.", "bulb"),
    ("sdd-design", "Design", "spec", "Diseña la solución y la arquitectura.", "blueprint"),
    ("sdd-tasks", "Tasks", "spec", "Descompone el diseño en tareas accionables.", "checklist"),
    ("sdd-apply", "Apply", "exec", "Implementa el código de las tareas.", "code"),
    ("sdd-verify", "Verify", "tdd", "Corre y valida tests contra el contrato.", "check"),
    ("sdd-judge", "Judge", "tdd", "Revisión adversarial de la implementación.", "scale"),
    ("sdd-archive", "Archive", "close", "Archiva el resultado y los artefactos.", "archive"),
    ("sdd-onboard", "Onboard", "close", "Documenta y deja onboarding del cambio.", "book"),
]


@csrf_protect
def sdd_flow(request):
    """Vista general del pipeline SDD como diagrama de loop.

    Resuelve los agent_templates por slug sdd-* (una sola query) y arma la lista
    ordenada de las 10 fases. Cada fase lleva el id del template si está seedeado
    (el nodo abre el modal de edición del prompt reusando agenttemplates); si no,
    el nodo se muestra deshabilitado.
    """
    redir = _require_auth(request)
    if redir:
        return redir

    # Import local: evita acoplar config a un app de mantenedor en import-time.
    from maintainers.agenttemplates.models import AgentTemplate

    slugs = [slug for slug, *_ in _SDD_PHASES]
    by_slug = {
        t.slug: t
        for t in AgentTemplate.objects.filter(slug__in=slugs).only("id", "slug", "name")
    }

    phases = []
    for index, (slug, name, group, desc, icon) in enumerate(_SDD_PHASES):
        tpl = by_slug.get(slug)
        phases.append(
            {
                "index": index,
                "step": index + 1,
                "slug": slug,
                "name": name,
                "group": group,
                "desc": desc,
                "icon": icon,
                "id": str(tpl.id) if tpl else None,
                "seeded": tpl is not None,
            }
        )

    return render(request, "sdd_flow.html", {"phases": phases})


def logout_view(request):
    request.session.flush()
    messages.info(request, "Sesión cerrada.")
    return HttpResponseRedirect("/login/")


# --- Error handlers (referenciados desde config.urls) ---
# Renderizan templates/errors/{code}.html con el status correcto.
# Firma: los handlers 400/403/404 reciben (request, exception); el 500 solo
# (request). Mantener los kwargs/firmas que Django espera.

def bad_request(request, exception=None):
    return render(request, "errors/400.html", status=400)


def permission_denied(request, exception=None):
    return render(request, "errors/403.html", status=403)


def page_not_found(request, exception=None):
    return render(request, "errors/404.html", status=404)


def server_error(request):
    return render(request, "errors/500.html", status=500)


urlpatterns = [
    path("", login_view, name="home"),
    path("login/", login_view, name="login"),
    path("dashboard/", dashboard, name="dashboard"),
    path("components/", components_demo, name="components"),
    path("logout/", logout_view, name="logout"),
]