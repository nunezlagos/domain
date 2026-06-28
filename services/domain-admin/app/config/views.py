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
    """Retorna redirect a /login/ si no esta autenticado, None si OK."""
    if not _is_authed(request):
        return HttpResponseRedirect("/login/")
    return None


def login_view(request):

    if _is_authed(request):
        return HttpResponseRedirect("/dashboard/")

    if request.method == "POST":
        email = request.POST.get("email", "").strip()
        password = request.POST.get("password", "")
        admin_email, admin_password = _admin_creds()
        if email == admin_email and password == admin_password:

            request.session.cycle_key()
            request.session["authenticated"] = True
            request.session["email"] = admin_email
            messages.success(request, f"Bienvenido, {admin_email}")
            return HttpResponseRedirect("/dashboard/")
        messages.error(request, "Email o contraseña incorrectos.")

    return render(request, "login.html")


def home_view(request):
    """Raiz: redirige al panel o al login."""
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







_SDD_PHASES = [
    ("sdd-explore", "Explore", "spec", "Mapea el contexto y el codigo existente.", "search"),
    ("sdd-spec", "Spec", "spec", "Define el contrato y los criterios de aceptacion.", "doc"),
    ("sdd-propose", "Propose", "spec", "Propone enfoques con sus tradeoffs.", "bulb"),
    ("sdd-design", "Design", "spec", "Diseña la solucion y la arquitectura.", "blueprint"),
    ("sdd-tasks", "Tasks", "spec", "Descompone el diseño en tareas accionables.", "checklist"),
    ("sdd-apply", "Apply", "exec", "Implementa el codigo de las tareas.", "code"),
    ("sdd-verify", "Verify", "tdd", "Corre y valida tests contra el contrato.", "check"),
    ("sdd-judge", "Judge", "tdd", "Revision adversarial de la implementacion.", "scale"),
    ("sdd-archive", "Archive", "close", "Archiva el resultado y los artefactos.", "archive"),
    ("sdd-onboard", "Onboard", "close", "Documenta y deja onboarding del cambio.", "book"),
]

_SDD_PHASE_OPS = {
    "sdd-explore": {
        "output": "intent + directorio → contexto mapeado",
        "tools_mcp": [
            "domain_mem_search → SELECT FTS en knowledge_observations",
            "domain_mem_save → INSERT knowledge_observations (intent)",
        ],
        "db_ops": [
            {"type": "read", "label": "knowledge_observations (BM25/FTS: plainto_tsquery spanish)"},
            {"type": "write", "label": "knowledge_observations (opcional)"},
        ],
    },
    "sdd-spec": {
        "output": "issue_slug + issue_md → especificación formal",
        "tools_mcp": [
            "domain_mem_search → SELECT FTS en knowledge_observations",
            "domain_knowledge_save → INSERT knowledge_docs + chunks (spec)",
            "domain_mem_save → INSERT knowledge_observations (issue slug)",
        ],
        "db_ops": [
            {"type": "read", "label": "knowledge_observations (FT: contexto similar)"},
            {"type": "write", "label": "knowledge_docs + doc_chunks (body del spec con embeddings)"},
            {"type": "write", "label": "knowledge_observations (opcional)"},
        ],
    },
    "sdd-propose": {
        "output": "proposal_md → enfoque con tradeoffs",
        "tools_mcp": [
            "domain_mem_search → SELECT FTS en knowledge_observations",
            "domain_propose_skill → INSERT project_skills (propuesta)",
            "domain_propose_policy → INSERT project_policies (propuesta)",
            "domain_knowledge_save → INSERT knowledge_docs + chunks (proposal)",
            "domain_mem_save → INSERT knowledge_observations (REQUERIDO)",
        ],
        "db_ops": [
            {"type": "read", "label": "knowledge_observations (BM25/FTS: enfoques similares)"},
            {"type": "write", "label": "knowledge_docs + doc_chunks (contenido de la propuesta)"},
            {"type": "write", "label": "knowledge_observations (observación REQUERIDA del análisis)"},
            {"type": "write", "label": "project_skills / project_policies (skills/policies propuestas)"},
        ],
    },
    "sdd-design": {
        "output": "design_md + ADRs → arquitectura detallada",
        "tools_mcp": [
            "domain_mem_search → SELECT FTS en knowledge_observations / docs",
            "domain_knowledge_save → INSERT knowledge_docs + chunks (design_md)",
            "domain_mem_save → INSERT knowledge_observations (ADRs REQUERIDOS)",
        ],
        "db_ops": [
            {"type": "read", "label": "knowledge_observations + knowledge_docs (BM25/FTS)"},
            {"type": "write", "label": "knowledge_docs + doc_chunks (diseño completo con embeddings)"},
            {"type": "write", "label": "knowledge_observations (ADRs como observaciones REQUERIDAS)"},
        ],
    },
    "sdd-tasks": {
        "output": "tasks[] → descomposición atómica (id + desc + effort + depends_on)",
        "tools_mcp": [
            "domain_mem_search → SELECT FTS en knowledge_observations",
            "domain_knowledge_save → INSERT knowledge_docs + chunks (task breakdown)",
            "domain_mem_save → INSERT knowledge_observations (REQUERIDO)",
        ],
        "db_ops": [
            {"type": "read", "label": "knowledge_observations (BM25/FTS)"},
            {"type": "write", "label": "knowledge_docs + doc_chunks (descomposición en tareas)"},
            {"type": "write", "label": "knowledge_observations (observación REQUERIDA)"},
        ],
    },
    "sdd-apply": {
        "output": "files_changed → implementación + code_reference",
        "tools_mcp": [
            "domain_mem_search → SELECT FTS en knowledge_observations",
            "domain_project_skill_list → SELECT project_skills",
            "domain_knowledge_save → INSERT knowledge_docs + chunks (code summary)",
            "domain_mem_save → INSERT knowledge_observations (code_reference REQUERIDO)",
        ],
        "db_ops": [
            {"type": "read", "label": "knowledge_observations + project_skills (BM25/FTS + skills)"},
            {"type": "write", "label": "knowledge_docs + doc_chunks (resumen de implementación)"},
            {"type": "write", "label": "knowledge_observations (code_reference REQUERIDO)"},
        ],
    },
    "sdd-verify": {
        "output": "scenarios_failed + blockers → validación contra contrato",
        "tools_mcp": [
            "Test runner local (TDD: red → green → refactor → sabotage)",
            "domain_mem_save → INSERT knowledge_observations (resultados opcional)",
        ],
        "db_ops": [
            {"type": "write", "label": "knowledge_observations (verification result opcional)"},
        ],
    },
    "sdd-judge": {
        "output": "sabotage_records[] → revisión adversarial",
        "tools_mcp": [
            "Subagentes paralelos nativos del cliente (adversarial review)",
            "domain_mem_save → INSERT knowledge_observations (sabotage_record REQUERIDO)",
        ],
        "db_ops": [
            {"type": "write", "label": "knowledge_observations (sabotage_record REQUERIDO)"},
        ],
    },
    "sdd-archive": {
        "output": "archived=true → issue cerrado + artefactos",
        "tools_mcp": [
            "domain_mem_save → INSERT knowledge_observations (resumen opcional)",
        ],
        "db_ops": [
            {"type": "write", "label": "flow_run_steps (archived=true)"},
            {"type": "write", "label": "knowledge_observations (resumen opcional)"},
        ],
    },
    "sdd-onboard": {
        "output": "doc_created (opcional) → onboarding del cambio",
        "tools_mcp": [
            "domain_knowledge_save → INSERT knowledge_docs + chunks (guía de onboarding)",
            "domain_mem_save → INSERT knowledge_observations (opcional)",
        ],
        "db_ops": [
            {"type": "write", "label": "knowledge_docs + doc_chunks (documentación de onboarding)"},
            {"type": "write", "label": "knowledge_observations (opcional)"},
        ],
    },
}

_SDD_PRE_OPS = [
    {"type": "read", "label": "flows (slug→id), agent_templates (system prompt), policies (platform + project)"},
    {"type": "write", "label": "flow_runs (nuevo run), flow_run_steps (N steps del plan)"},
]


@csrf_protect
def sdd_flow(request):
    """Vista general del pipeline SDD como diagrama de loop.

    Resuelve los agent_templates por slug sdd-* (una sola query) y arma la lista
    ordenada de las 10 fases. Cada fase lleva el id del template si esta seedeado
    (el nodo abre el modal de edicion del prompt reusando agenttemplates); si no,
    el nodo se muestra deshabilitado.
    """
    redir = _require_auth(request)
    if redir:
        return redir


    from maintainers.agenttemplates.models import AgentTemplate

    slugs = [slug for slug, *_ in _SDD_PHASES]
    by_slug = {
        t.slug: t
        for t in AgentTemplate.objects.filter(slug__in=slugs).only("id", "slug", "name")
    }







    _LITE = {"sdd-explore", "sdd-apply", "sdd-verify"}
    _EXPRESS = {"sdd-apply", "sdd-verify"}
    _GATE = {"sdd-spec", "sdd-design", "sdd-apply", "sdd-judge"}
    _HARDSPEC = {"sdd-spec"}
    _LOOP = {"sdd-apply", "sdd-verify", "sdd-judge"}

    phases = []
    for index, (slug, name, group, desc, icon) in enumerate(_SDD_PHASES):
        tpl = by_slug.get(slug)
        ops = _SDD_PHASE_OPS.get(slug, {})
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
                "lite": slug in _LITE,
                "express": slug in _EXPRESS,
                "gate": slug in _GATE,
                "hardspec": slug in _HARDSPEC,
                "loop": slug in _LOOP,
                "output": ops.get("output", "resultado"),
                "tools_mcp": ops.get("tools_mcp", []),
                "db_ops": ops.get("db_ops", []),
            }
        )

    pmap = {p["slug"].removeprefix("sdd-"): p for p in phases}

    return render(request, "sdd_flow.html", {"phases": phases, "pmap": pmap, "pre_ops": _SDD_PRE_OPS})


def logout_view(request):
    request.session.flush()
    messages.info(request, "Sesion cerrada.")
    return HttpResponseRedirect("/login/")







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