"""HU-47.2: views del admin dashboard.

Single-user hardcoded auth via env vars. CSRF + Messages activados.
"""
import json
import logging
import os

from django.conf import settings
from django.contrib import messages
from django.http import HttpResponseRedirect, JsonResponse
from django.shortcuts import render
from django.urls import path
from django.views.decorators.csrf import csrf_protect
from django.views.decorators.http import require_POST

logger = logging.getLogger(__name__)


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

    return render(request, "login.html", {
        "google_client_id": settings.GOOGLE_CLIENT_ID,
    })


@require_POST
@csrf_protect
def google_login(request):
    """Verifica credential de Google Sign-In y autentica al admin."""
    client_id = settings.GOOGLE_CLIENT_ID
    if not client_id:
        return JsonResponse({"error": "Google login no configurado"}, status=400)

    credential = request.POST.get("credential", "")
    if not credential:
        return JsonResponse({"error": "Credencial vacía"}, status=400)

    try:
        from google.oauth2 import id_token
        from google.auth.transport import requests

        info = id_token.verify_oauth2_token(credential, requests.Request(), client_id)
        email = info.get("email", "")
        admin_email, _ = _admin_creds()
        if email != admin_email:
            logger.warning("google_login: email %s no es el admin (%s)", email, admin_email)
            messages.error(request, f"La cuenta {email} no tiene acceso.")
            return HttpResponseRedirect("/login/")

        request.session.cycle_key()
        request.session["authenticated"] = True
        request.session["email"] = admin_email
        messages.success(request, f"Bienvenido, {admin_email}")
        return HttpResponseRedirect("/dashboard/")
    except ValueError as e:
        logger.error("google_login: token inválido: %s", e)
        messages.error(request, "Token de Google inválido o expirado.")
        return HttpResponseRedirect("/login/")


def home_view(request):
    """Raiz: landing pública o panel si ya está autenticado."""
    if _is_authed(request):
        return HttpResponseRedirect("/dashboard/")
    return render(request, "landing.html")


def _build_portal_ctx():
    """Construye el contexto del portal con datos reales de todos los modelos."""
    from datetime import timedelta

    from django.utils import timezone

    from maintainers.agents.models import Agent
    from maintainers.crons.models import Cron
    from maintainers.feedback.models import SkillFeedback
    from maintainers.flows.models import Flow
    from maintainers.platformpolicies.models import PlatformPolicy
    from maintainers.projectpolicies.models import ProjectPolicy
    from maintainers.projects.models import Project
    from maintainers.prompts.models import Prompt
    from maintainers.skills.models import Skill
    from maintainers.users.models import User

    now = timezone.now()
    week_ago = now - timedelta(days=7)

    def _q(fn):
        try:
            return fn()
        except Exception:
            return []

    def _c(fn):
        try:
            return fn()
        except Exception:
            return 0

    def _fmt_dt(dt):
        if not dt:
            return "nunca"
        delta = now - dt
        if delta.seconds < 60:
            return "ahora"
        if delta.days == 0 and delta.seconds < 3600:
            return f"hace {delta.seconds // 60}m"
        if delta.days == 0:
            return f"hace {delta.seconds // 3600}h"
        return f"hace {delta.days}d"

    agents = _q(lambda: [
        {"name": a.name, "slug": a.slug, "provider": a.provider,
         "model": a.model, "status": a.status, "calls": 0}
        for a in Agent.objects.filter(deleted_at__isnull=True).order_by("-created_at")
    ])

    skills = _q(lambda: [
        {"name": s.name, "slug": s.slug, "type": s.skill_type,
         "desc": (s.description or "")[:80], "status": s.status, "calls": 0, "success": 100}
        for s in Skill.objects.filter(deleted_at__isnull=True).order_by("-created_at")
    ])

    flows = _q(lambda: [
        {"name": f.name, "slug": f.slug,
         "phases": len(f.spec.get("phases", [])) if isinstance(f.spec, dict) else 0,
         "status": "active" if f.is_active else "inactive", "runs": 0}
        for f in Flow.objects.filter(deleted_at__isnull=True).order_by("-created_at")
    ])

    prompts = _q(lambda: [
        {"name": p.slug, "slug": p.slug, "model": "",
         "status": "active" if p.is_active else "inactive", "uses": 0}
        for p in Prompt.objects.filter(deleted_at__isnull=True).order_by("-created_at")
    ])

    crons = _q(lambda: [
        {"name": c.name, "slug": c.slug, "schedule": c.cron_expression,
         "status": c.status if c.status in ("active", "inactive")
                  else ("active" if c.enabled else "inactive"),
         "last_run": _fmt_dt(c.last_run_at)}
        for c in Cron.objects.filter(deleted_at__isnull=True).order_by("-created_at")
    ])

    projects = _q(lambda: [
        {"name": p.name, "slug": p.slug, "status": p.status,
         "skills": 0, "agents": 0, "flows": 0}
        for p in Project.objects.filter(deleted_at__isnull=True).order_by("-created_at")
    ])

    users = _q(lambda: [
        {"name": u.name or u.email, "email": u.email, "role": u.role, "status": u.status}
        for u in User.objects.filter(deleted_at__isnull=True).order_by("-created_at")
    ])

    proj_policies = _q(lambda: [
        {"name": p.name, "slug": p.slug, "scope": "project", "kind": p.kind,
         "status": "active" if p.is_active else "inactive"}
        for p in ProjectPolicy.objects.filter(deleted_at__isnull=True).order_by("-created_at")
    ])
    plat_policies = _q(lambda: [
        {"name": p.name, "slug": p.slug, "scope": "platform", "kind": p.kind,
         "status": "active" if p.is_active else "inactive"}
        for p in PlatformPolicy.objects.all().order_by("-created_at")
    ])
    policies = proj_policies + plat_policies

    feedback_pos   = _c(lambda: SkillFeedback.objects.filter(rating=1,  created_at__gte=week_ago).count())
    feedback_neg   = _c(lambda: SkillFeedback.objects.filter(rating=-1, created_at__gte=week_ago).count())
    feedback_total = feedback_pos + feedback_neg
    feedback_pct   = round(feedback_pos / feedback_total * 100, 1) if feedback_total > 0 else 0.0

    agent_count   = sum(1 for a in agents   if a["status"] == "active")
    skill_count   = sum(1 for s in skills   if s["status"] == "active")
    project_count = sum(1 for p in projects if p["status"] == "active")
    flow_count    = sum(1 for f in flows    if f["status"] == "active")

    portal_data = {
        "agents": agents, "skills": skills, "flows": flows,
        "prompts": prompts, "crons": crons, "projects": projects,
        "users": users, "policies": policies,
    }

    return {
        "portal_data_json": json.dumps(portal_data, default=str),
        "feedback_pos":     feedback_pos,
        "feedback_neg":     feedback_neg,
        "feedback_pct":     feedback_pct,
        "agent_count":      agent_count,
        "skill_count":      skill_count,
        "project_count":    project_count,
        "flow_count":       flow_count,
    }


@csrf_protect
def dashboard(request):
    redir = _require_auth(request)
    if redir:
        return redir
    return render(request, "portal.html", _build_portal_ctx())


@csrf_protect
def components_demo(request):
    redir = _require_auth(request)
    if redir:
        return redir
    return render(request, "components_demo.html")







_SDD_PHASES = [
    ("sdd-explore", "Explore", "spec", "Mapea el contexto y el codigo existente.", "magnifying-glass"),
    ("sdd-spec", "Spec", "spec", "Define el contrato y los criterios de aceptacion.", "file-lines"),
    ("sdd-propose", "Propose", "spec", "Propone enfoques con sus tradeoffs.", "lightbulb"),
    ("sdd-design", "Design", "spec", "Diseña la solucion y la arquitectura.", "compass-drafting"),
    ("sdd-tasks", "Tasks", "spec", "Descompone el diseño en tareas accionables.", "list-check"),
    ("sdd-apply", "Apply", "exec", "Implementa el codigo de las tareas.", "code"),
    ("sdd-verify", "Verify", "tdd", "Corre y valida tests contra el contrato.", "shield-halved"),
    ("sdd-judge", "Judge", "tdd", "Revision adversarial de la implementacion.", "gavel"),
    ("sdd-review", "Review", "tdd", "Audita el cambio contra policies y skills del proyecto.", "clipboard-check"),
    ("sdd-archive", "Archive", "close", "Archiva el resultado y los artefactos.", "box-archive"),
    ("sdd-onboard", "Onboard", "close", "Documenta y deja onboarding del cambio.", "rocket"),
]

_SDD_PHASE_OPS = {
    "sdd-explore": {
        "output": "intent + scope + affected_paths → contexto mapeado desde el code graph",
        "tools_mcp": [
            "domain_code_graph → grafo de código vivo (REQUERIDO)",
            "domain_code_explore → navegación de nodos/edges del grafo",
            "domain_mem_search → SELECT FTS en knowledge_observations",
        ],
        "db_ops": [
            {"type": "read", "label": "code_nodes + code_edges (grafo de código)"},
            {"type": "read", "label": "knowledge_observations (BM25/FTS: plainto_tsquery spanish)"},
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
            {"type": "save", "label": "sdd_proposals (auto: ParseProposal → CreateProposal)"},
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
            {"type": "save", "label": "sdd_designs (auto: ParseDesign → CreateDesign)"},
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
            {"type": "save", "label": "issue_tasks (auto: CreateTasks)"},
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
    "sdd-review": {
        "output": "verdict: compliant | violations_found → auditoría de policies",
        "tools_mcp": [
            "domain_project_policy_list → SELECT project_policies (resolver project→platform)",
            "domain_verify_start / domain_verify_update_item / domain_verify_complete → checkpoint en tdd_verifications",
        ],
        "db_ops": [
            {"type": "read", "label": "project_policies + project_skills (resolver jerárquico)"},
            {"type": "write", "label": "tdd_verifications (checkpoint de review; violations_found bloquea archive)"},
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

# Modos REALES del orquestador (services/domain-mcp/internal/service/orchestrator/modes).
# full/async/detect ejecutan las 11 fases; lite = explore+apply+verify; express = apply+verify;
# solo = ejecución server-side inline (sin desglose de fases cliente).
# hybrid y manual NO son modos: son exec_modes (controlan dónde PAUSA el flujo, no qué fases
# corre). Por eso no aparecen como workflow tabs.
_SDD_FULL_PHASES = ["sdd-explore", "sdd-spec", "sdd-propose", "sdd-design", "sdd-tasks", "sdd-apply", "sdd-verify", "sdd-judge", "sdd-review", "sdd-archive", "sdd-onboard"]
_SDD_WORKFLOWS = [
    {"slug": "full",    "name": "Full",    "phases": _SDD_FULL_PHASES},
    {"slug": "lite",    "name": "Lite",    "phases": ["sdd-explore", "sdd-apply", "sdd-verify"]},
    {"slug": "express", "name": "Express", "phases": ["sdd-apply", "sdd-verify"]},
    {"slug": "async",   "name": "Async",   "phases": _SDD_FULL_PHASES},
    {"slug": "detect",  "name": "Detect",  "phases": _SDD_FULL_PHASES},
    {"slug": "solo",    "name": "Solo",    "phases": _SDD_FULL_PHASES},
]


@csrf_protect
def sdd_flow(request):
    """Vista general del pipeline SDD como grafo de nodos.

    Resuelve los agent_templates por slug sdd-* (una sola query) y arma la lista
    ordenada de las 11 fases. Cada nodo lleva el id del template si esta seedeado.
    Workflow tabs filtran las fases por modo de ejecucion (full/lite/express/async/
    detect/solo). hybrid y manual son exec_modes, no modos, por eso no son tabs.
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

    # Build workflow-phase mapping: for each phase, which workflows include it
    wf_phase_map = {}
    for wf in _SDD_WORKFLOWS:
        for ps in wf["phases"]:
            wf_phase_map.setdefault(ps, []).append(wf["slug"])

    # Pre-serialize workflows with JSON phases list for JS
    for wf in _SDD_WORKFLOWS:
        wf["phases_json"] = json.dumps(wf["phases"])

    phases = []
    for index, (slug, name, group, desc, icon) in enumerate(_SDD_PHASES):
        tpl = by_slug.get(slug)
        ops = _SDD_PHASE_OPS.get(slug, {})
        tools_mcp = ops.get("tools_mcp", [])
        db_ops = ops.get("db_ops", [])
        phases.append(
            {
                "slug": slug,
                "name": name,
                "desc": desc,
                "icon": icon,
                "agent_slug": slug,
                "id": str(tpl.id) if tpl else None,
                "seeded": tpl is not None,
                "output": ops.get("output", "resultado"),
                "tools_mcp": tools_mcp,
                "tools_mcp_json": json.dumps(tools_mcp),
                "db_ops": db_ops,
                "db_ops_json": json.dumps(db_ops),
                "modes": ",".join(wf_phase_map.get(slug, ["full"])),
            }
        )

    phases_json = json.dumps([
        {"id": p["slug"], "name": p["name"], "desc": p["desc"],
         "icon": p["icon"], "output": p["output"],
         "tools": p["tools_mcp"], "ops": p["db_ops"]}
        for p in phases
    ])

    return render(request, "sdd_flow.html", {
        "phases": phases,
        "phases_json": phases_json,
        "workflows": _SDD_WORKFLOWS,
    })


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