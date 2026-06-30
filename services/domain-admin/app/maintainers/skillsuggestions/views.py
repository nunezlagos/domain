"""HU-52.3: views de la app skillsuggestions (LLM-as-judge, human-in-the-loop).

Superficies:
  GET  /skill-suggestions/                 lista (filtros status/kind/skill)
  GET  /skill-suggestions/<id>/            detalle (diff + rationale + payload)
  POST /skill-suggestions/<id>/approve     pending  -> approved   (regla dura 6)
  POST /skill-suggestions/<id>/reject      pending  -> rejected
  POST /skill-suggestions/<id>/apply       approved -> applied     (muta skills)

approve/reject/apply NO mutan por ORM: delegan al domain-mcp (Go), que es la
fuente de verdad de la transicion + el audit. El browser habla solo con Django
(sesion + CSRF); el Bearer al Go vive en el server (ver services.py).

El "comentario" del reviewer se persiste como un flash + se loguea (el endpoint
Go actual no recibe comentario en el body; ver nota en el codigo). approve/apply
son acciones distintas y deliberadamente separadas: approve no aplica.
"""
from __future__ import annotations

import logging

from django.contrib import messages
from django.http import HttpRequest, HttpResponse, Http404
from django.shortcuts import redirect, render
from django.views.decorators.http import require_http_methods

from core.auth import require_auth

from . import services
from .models import SkillSuggestion

log = logging.getLogger(__name__)


def _normalize_status(raw: str) -> str | None:
    raw = (raw or "").strip().lower()
    if raw in services.VALID_STATUSES:
        return raw
    return None


def _normalize_kind(raw: str) -> str | None:
    raw = (raw or "").strip().lower()
    if raw in services.VALID_KINDS:
        return raw
    return None


@require_http_methods(["GET"])
def admin_list(request: HttpRequest) -> HttpResponse:
    """Lista de sugerencias con filtros (default: pending)."""
    if (redir := require_auth(request)):
        return redir

    raw_status = request.GET.get("status")
    # Default a 'pending' solo cuando no se pidio nada explicito; 'all' = sin filtro.
    if raw_status is None:
        status = SkillSuggestion.STATUS_PENDING
        status_filter = SkillSuggestion.STATUS_PENDING
    elif raw_status.strip().lower() in ("", "all", "todas", "todos"):
        status = None
        status_filter = "all"
    else:
        status = _normalize_status(raw_status)
        status_filter = status or "all"

    kind = _normalize_kind(request.GET.get("kind", ""))
    skill_slug = (request.GET.get("skill") or "").strip()

    items = services.list_suggestions(
        status=status, kind=kind, skill_slug=skill_slug or None
    )

    return render(
        request,
        "skillsuggestions/list.html",
        {
            "items": items,
            "status_filter": status_filter,
            "kind_filter": kind or "all",
            "skill_filter": skill_slug,
            "pending_count": services.count_pending(),
            "kind_breakdown": services.kind_breakdown(),
            "kinds": services.VALID_KINDS,
        },
    )


@require_http_methods(["GET"])
def detail(request: HttpRequest, suggestion_id) -> HttpResponse:
    """Detalle: rationale + diff normalizado + payload crudo + metadata LLM."""
    if (redir := require_auth(request)):
        return redir

    suggestion = services.get_suggestion(suggestion_id)
    if suggestion is None:
        raise Http404("sugerencia no encontrada")

    diff = services.extract_diff(suggestion)

    return render(
        request,
        "skillsuggestions/detail.html",
        {
            "suggestion": suggestion,
            "diff": diff,
        },
    )


def _transition(
    request: HttpRequest,
    suggestion_id,
    *,
    action: str,
):
    """Ejecuta approve/reject/apply delegando al Go y traduce el resultado a flash."""
    suggestion = services.get_suggestion(suggestion_id)
    if suggestion is None:
        raise Http404("sugerencia no encontrada")

    comment = (request.POST.get("comment") or "").strip()
    fn = {"approve": services.approve, "reject": services.reject, "apply": services.apply}[action]
    label = {"approve": "aprobada", "reject": "rechazada", "apply": "aplicada"}[action]

    try:
        fn(suggestion_id)
    except services.ApiNotConfiguredError as exc:
        messages.error(request, str(exc))
        log.warning("transicion %s no configurada (id=%s)", action, suggestion_id)
        return redirect("skillsuggestions:detail", suggestion_id=suggestion_id)
    except services.SuggestionApiError as exc:
        # Mapeo legible de los conflictos de estado del Go a mensajes de usuario.
        human = {
            "not_pending": "La sugerencia ya fue revisada (no esta pendiente).",
            "not_approved": "La sugerencia no esta aprobada: no se puede aplicar.",
            "already_applied": "La sugerencia ya fue aplicada.",
            "seed_managed": "No se puede archivar/dividir un skill seed_managed.",
            "skill_not_found": "El skill objetivo no existe o fue borrado.",
            "apply_unavailable": "Apply requiere LLM (MINIMAX_API_KEY) o content en el payload.",
            "unauthorized": "El token del admin no esta autorizado para revisar (revisar DOMAIN_API_TOKEN).",
            "invalid_id": "Identificador de sugerencia invalido.",
            "suggestions_disabled": "El servicio de sugerencias no esta habilitado en el domain-mcp.",
        }.get(exc.code, str(exc))
        messages.error(request, human)
        log.warning(
            "transicion %s fallo (id=%s status=%s code=%s)",
            action, suggestion_id, exc.status, exc.code or "-",
        )
        return redirect("skillsuggestions:detail", suggestion_id=suggestion_id)

    # El comentario del reviewer no lo recibe el endpoint Go actual; lo dejamos
    # en el log y en el flash para trazabilidad humana (no es PII de terceros).
    if comment:
        log.info("skill-suggestion %s %s comentario=%r", suggestion_id, action, comment[:280])
    messages.success(request, f"Sugerencia {label}.")
    return redirect("skillsuggestions:detail", suggestion_id=suggestion_id)


@require_http_methods(["POST"])
def approve_view(request: HttpRequest, suggestion_id) -> HttpResponse:
    if (redir := require_auth(request)):
        return redir
    return _transition(request, suggestion_id, action="approve")


@require_http_methods(["POST"])
def reject_view(request: HttpRequest, suggestion_id) -> HttpResponse:
    if (redir := require_auth(request)):
        return redir
    return _transition(request, suggestion_id, action="reject")


@require_http_methods(["POST"])
def apply_view(request: HttpRequest, suggestion_id) -> HttpResponse:
    if (redir := require_auth(request)):
        return redir
    return _transition(request, suggestion_id, action="apply")
