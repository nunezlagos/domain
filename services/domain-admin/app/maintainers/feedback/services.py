"""HU-52.1: logica del feedback loop (👍/👎).

El widget del chat (browser, sesion del Django) hace POST a una view del
Django que llama a `submit_feedback`. Escribimos directo a `skill_feedback`
via ORM, igual que el chat escribe `chat_messages` directo (misma Postgres,
managed=False). Esto evita exponer el Bearer token del domain-mcp al browser:
el endpoint REST del Go (POST /api/v1/feedback) queda para integraciones
server-to-server; el browser usa la sesion del admin.

Idempotencia: 1 feedback por message_id (UNIQUE). Segundo submit = UPDATE
(upsert) — el usuario puede cambiar de opinion 👍 <-> 👎.

Privacy: `comment` puede tener PII -> NUNCA se loguea. Solo el rating
numerico y el message_id se consideran no-sensibles.
"""
from __future__ import annotations

import logging

from django.db import connection
from django.utils import timezone

from .models import SkillFeedback

log = logging.getLogger(__name__)

VALID_RATINGS = (SkillFeedback.RATING_UP, SkillFeedback.RATING_DOWN)


class InvalidRatingError(ValueError):
    """rating fuera de {1, -1}."""


class InvalidMessageError(ValueError):
    """message_id ausente o invalido."""


def submit_feedback(
    *,
    message_id: int,
    rating: int,
    skill_slug: str = "",
    comment: str = "",
    user_email: str = "",
) -> SkillFeedback:
    """Crea o actualiza (upsert) el feedback de un mensaje del assistant.

    Raises:
        InvalidRatingError: si rating no es 1 ni -1.
        InvalidMessageError: si message_id no es un entero positivo.
    """
    if rating not in VALID_RATINGS:
        raise InvalidRatingError("rating debe ser 1 o -1")
    try:
        mid = int(message_id)
    except (TypeError, ValueError):
        raise InvalidMessageError("message_id requerido") from None
    if mid <= 0:
        raise InvalidMessageError("message_id requerido")

    skill_slug = (skill_slug or "").strip() or None
    comment = (comment or "").strip() or None
    user_email = (user_email or "").strip() or None

    now = timezone.now()
    fb, created = SkillFeedback.objects.update_or_create(
        message_id=mid,
        defaults={
            "rating": rating,
            "skill_slug": skill_slug,
            "comment": comment,
            "user_email": user_email,
            "updated_at": now,
        },
    )
    # NUNCA loguear `comment` (PII). Solo metadata no sensible.
    log.info(
        "feedback %s msg#%s rating=%s skill=%s",
        "creado" if created else "actualizado",
        mid,
        rating,
        skill_slug or "-",
    )
    return fb


def list_feedback(rating: int | None = None, limit: int = 200) -> list[SkillFeedback]:
    """Lista feedback para la vista admin, opcionalmente filtrado por rating."""
    qs = SkillFeedback.objects.all()
    if rating in VALID_RATINGS:
        qs = qs.filter(rating=rating)
    return list(qs.order_by("-created_at")[:limit])


def aggregate_by_skill(rating: int | None = None) -> list[dict]:
    """Agregados por skill_slug desde skill_feedback (count_up/count_down).

    Self-contained: lee de skill_feedback directo (no depende de
    skill_feedback_daily ni del cron del Go). Sirve para la vista admin que
    muestra el resumen por skill. El filtro `rating` afecta la lista de filas
    crudas, no los conteos (los conteos siempre reflejan el total real).
    """
    sql = """
        SELECT
            COALESCE(NULLIF(skill_slug, ''), '(sin skill)')   AS skill_slug,
            COUNT(*) FILTER (WHERE rating = 1)                 AS count_up,
            COUNT(*) FILTER (WHERE rating = -1)                AS count_down,
            COUNT(*)                                           AS total,
            MAX(created_at)                                    AS last_feedback_at
        FROM skill_feedback
        GROUP BY COALESCE(NULLIF(skill_slug, ''), '(sin skill)')
        ORDER BY total DESC, skill_slug ASC
    """
    with connection.cursor() as cur:
        cur.execute(sql)
        cols = [c.name for c in cur.description]
        rows = [dict(zip(cols, r)) for r in cur.fetchall()]
    for r in rows:
        total = r["total"] or 0
        r["pct_up"] = round((r["count_up"] / total) * 100, 1) if total else 0.0
    return rows
