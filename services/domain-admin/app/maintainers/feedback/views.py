"""HU-52.1: views del feedback loop.

Dos superficies:
1. POST /feedback/api/submit  -> recibe el voto 👍/👎 del widget del chat.
   CSRF-exempt (fetch del browser con sesion del admin), session-auth,
   rate limit 30/min por user_email (anti-spam, espeja el limite del Go).
   Escribe directo a skill_feedback (upsert por message_id).
2. GET  /feedback/            -> vista admin: lista de feedback + resumen por
   skill, con filtro por rating (positive/negative).

NO usa el endpoint REST del Go desde el browser: el browser no tiene el
Bearer token del domain-mcp. El POST REST del Go (/api/v1/feedback) queda
para integraciones server-to-server.
"""
from __future__ import annotations

import json
import logging

from django.http import HttpRequest, HttpResponse, JsonResponse
from django.shortcuts import render
from django.views.decorators.csrf import csrf_exempt
from django.views.decorators.http import require_http_methods

from core.auth import require_auth

from . import services
from .rate_limit import check_feedback

log = logging.getLogger(__name__)


def _current_email(request: HttpRequest) -> str:
    return request.session.get("email", "anonymous@local")


def _parse_body(request: HttpRequest) -> dict | None:
    if not request.body:
        return None
    try:
        return json.loads(request.body)
    except json.JSONDecodeError:
        return None


@csrf_exempt
@require_http_methods(["POST"])
def submit(request: HttpRequest) -> JsonResponse:
    """Recibe el voto del widget. Body: message_id, rating, comment, skill_slug."""
    redir = require_auth(request)
    if redir:
        return redir

    email = _current_email(request)
    rl = check_feedback(email)
    if not rl.allowed:
        return JsonResponse(
            {
                "error": "rate_limited",
                "message": f"demasiados feedbacks; intenta en {rl.reset_in_seconds}s",
                "reset_in": rl.reset_in_seconds,
            },
            status=429,
        )

    body = _parse_body(request)
    if not body:
        return JsonResponse({"error": "body_invalido"}, status=400)

    try:
        fb = services.submit_feedback(
            message_id=body.get("message_id"),
            rating=body.get("rating"),
            skill_slug=body.get("skill_slug", ""),
            comment=body.get("comment", ""),
            user_email=email,
        )
    except services.InvalidRatingError:
        return JsonResponse({"error": "invalid_rating", "message": "rating debe ser 1 o -1"}, status=400)
    except services.InvalidMessageError:
        return JsonResponse({"error": "invalid_message", "message": "message_id requerido"}, status=400)

    return JsonResponse(
        {
            "data": {
                "id": str(fb.id),
                "message_id": fb.message_id,
                "rating": fb.rating,
                "skill_slug": fb.skill_slug or "",
            }
        },
        status=200,
    )


@require_http_methods(["GET"])
def admin_list(request: HttpRequest) -> HttpResponse:
    """Vista admin: lista de feedback + resumen por skill, filtro por rating."""
    redir = require_auth(request)
    if redir:
        return redir

    raw = (request.GET.get("rating") or "").strip().lower()
    if raw in ("1", "positive", "up"):
        rating = services.SkillFeedback.RATING_UP
        rating_filter = "positive"
    elif raw in ("-1", "negative", "down"):
        rating = services.SkillFeedback.RATING_DOWN
        rating_filter = "negative"
    else:
        rating = None
        rating_filter = "all"

    items = services.list_feedback(rating=rating)
    summary = services.aggregate_by_skill()

    return render(
        request,
        "feedback/list.html",
        {
            "items": items,
            "summary": summary,
            "rating_filter": rating_filter,
        },
    )
