"""Queries SQL para el dashboard de uso.

Todas las consultas se ejecutan dentro de una transaccion con
SET LOCAL app.current_org_id para cumplir con el FORCE RLS de
la tabla captured_prompts.

DEFAULT_ORG_ID se lee de settings; si no esta configurado usa el
UUID canonico del single-org deployment (00000000-0000-0000-0000-000000000001).
"""
from __future__ import annotations

from django.conf import settings
from django.db import connection, transaction


def _org_id() -> str:
    return getattr(settings, "DEFAULT_ORG_ID", "00000000-0000-0000-0000-000000000001")


def _set_org(cursor) -> None:
    cursor.execute("SET LOCAL app.current_org_id = %s", [_org_id()])


def kpis(days: int = 30) -> dict:
    sql = """
        SELECT
            COUNT(*)                                       AS total_turns,
            COUNT(*) FILTER (WHERE turn_completed_at IS NOT NULL)
                                                           AS completed_turns,
            COALESCE(SUM(estimated_tokens_in),  0)         AS tokens_in,
            COALESCE(SUM(estimated_tokens_out), 0)         AS tokens_out,
            COALESCE(SUM(estimated_tokens_in + estimated_tokens_out), 0) AS tokens_total,
            COALESCE(AVG(estimated_tokens_in + estimated_tokens_out)
                FILTER (WHERE turn_completed_at IS NOT NULL), 0)
                                                           AS avg_tokens_turn
        FROM captured_prompts
        WHERE captured_at >= NOW() - (%s || ' days')::INTERVAL
    """
    with transaction.atomic():
        with connection.cursor() as cur:
            _set_org(cur)
            cur.execute(sql, [str(days)])
            row = cur.fetchone()
    return {
        "total_turns":     int(row[0]),
        "completed_turns": int(row[1]),
        "tokens_in":       int(row[2]),
        "tokens_out":      int(row[3]),
        "tokens_total":    int(row[4]),
        "avg_tokens_turn": round(float(row[5]), 1),
    }


def by_project(days: int = 30) -> list[dict]:
    sql = """
        SELECT
            COALESCE(p.slug, '(sin proyecto)')             AS project_slug,
            COUNT(cp.id)                                   AS turns,
            COALESCE(SUM(cp.estimated_tokens_in),  0)      AS tokens_in,
            COALESCE(SUM(cp.estimated_tokens_out), 0)      AS tokens_out,
            COALESCE(SUM(cp.estimated_tokens_in + cp.estimated_tokens_out), 0)
                                                           AS tokens_total,
            MAX(cp.captured_at)                            AS last_turn
        FROM captured_prompts cp
        LEFT JOIN projects p ON p.id = cp.project_id
        WHERE cp.captured_at >= NOW() - (%s || ' days')::INTERVAL
        GROUP BY p.slug
        ORDER BY tokens_total DESC
        LIMIT 50
    """
    with transaction.atomic():
        with connection.cursor() as cur:
            _set_org(cur)
            cur.execute(sql, [str(days)])
            cols = [c.name for c in cur.description]
            rows = cur.fetchall()
    return [dict(zip(cols, r)) for r in rows]


def by_client(days: int = 30) -> list[dict]:
    sql = """
        SELECT
            COALESCE(NULLIF(client_kind, ''), '(desconocido)') AS client,
            COUNT(*)                                            AS turns,
            COALESCE(SUM(estimated_tokens_in),  0)             AS tokens_in,
            COALESCE(SUM(estimated_tokens_out), 0)             AS tokens_out,
            COALESCE(SUM(estimated_tokens_in + estimated_tokens_out), 0)
                                                               AS tokens_total
        FROM captured_prompts
        WHERE captured_at >= NOW() - (%s || ' days')::INTERVAL
        GROUP BY client_kind
        ORDER BY tokens_total DESC
    """
    with transaction.atomic():
        with connection.cursor() as cur:
            _set_org(cur)
            cur.execute(sql, [str(days)])
            cols = [c.name for c in cur.description]
            rows = cur.fetchall()
    return [dict(zip(cols, r)) for r in rows]


def by_model(days: int = 30) -> list[dict]:
    sql = """
        SELECT
            COALESCE(NULLIF(model, ''), '(desconocido)') AS model,
            COUNT(*)                                      AS turns,
            COALESCE(SUM(estimated_tokens_in),  0)        AS tokens_in,
            COALESCE(SUM(estimated_tokens_out), 0)        AS tokens_out
        FROM captured_prompts
        WHERE captured_at >= NOW() - (%s || ' days')::INTERVAL
        GROUP BY model
        ORDER BY turns DESC
    """
    with transaction.atomic():
        with connection.cursor() as cur:
            _set_org(cur)
            cur.execute(sql, [str(days)])
            cols = [c.name for c in cur.description]
            rows = cur.fetchall()
    return [dict(zip(cols, r)) for r in rows]


def recent_prompts(days: int = 30, limit: int = 50) -> list[dict]:
    sql = """
        SELECT
            cp.id,
            cp.captured_at,
            COALESCE(p.slug, '—')                       AS project_slug,
            COALESCE(NULLIF(cp.client_kind, ''), '—')   AS client_kind,
            COALESCE(NULLIF(cp.model, ''), '—')         AS model,
            cp.estimated_tokens_in,
            cp.estimated_tokens_out,
            cp.estimated_tokens_in + cp.estimated_tokens_out
                                                        AS tokens_total,
            cp.turn_completed_at,
            LEFT(cp.content, 140)                       AS content_preview
        FROM captured_prompts cp
        LEFT JOIN projects p ON p.id = cp.project_id
        WHERE cp.captured_at >= NOW() - (%s || ' days')::INTERVAL
        ORDER BY cp.captured_at DESC
        LIMIT %s
    """
    with transaction.atomic():
        with connection.cursor() as cur:
            _set_org(cur)
            cur.execute(sql, [str(days), limit])
            cols = [c.name for c in cur.description]
            rows = cur.fetchall()
    return [dict(zip(cols, r)) for r in rows]
