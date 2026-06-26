"""Queries read-only para la vista de uptime/health del MCP.

La tabla `mcp_health_checks` es metrica de plataforma (sin org / sin RLS),
asi que NO necesita SET LOCAL app.current_org_id (a diferencia de usage).

Uptime % = chequeos con status='up' / total de chequeos en la ventana.
Los chequeos 'degraded' NO cuentan como up (es disponibilidad estricta).
"""
from __future__ import annotations

from django.db import connection


def _rows(sql: str, params: list) -> list[dict]:
    with connection.cursor() as cur:
        cur.execute(sql, params)
        cols = [c.name for c in cur.description]
        return [dict(zip(cols, r)) for r in cur.fetchall()]


def _one(sql: str, params: list) -> tuple | None:
    with connection.cursor() as cur:
        cur.execute(sql, params)
        return cur.fetchone()


def uptime_window(hours: int) -> dict:
    """Uptime % + conteos + latencia promedio en una ventana de N horas."""
    sql = """
        SELECT
            COUNT(*)                                           AS total,
            COUNT(*) FILTER (WHERE status = 'up')              AS up,
            COUNT(*) FILTER (WHERE status = 'degraded')        AS degraded,
            COUNT(*) FILTER (WHERE status = 'down')            AS down,
            COALESCE(AVG(latency_ms) FILTER (WHERE status = 'up'), 0) AS avg_latency_ms
        FROM mcp_health_checks
        WHERE checked_at >= NOW() - (%s || ' hours')::INTERVAL
    """
    row = _one(sql, [str(hours)])
    total = int(row[0]) if row else 0
    up = int(row[1]) if row else 0
    degraded = int(row[2]) if row else 0
    down = int(row[3]) if row else 0
    avg_latency = round(float(row[4]), 1) if row else 0.0
    uptime_pct = round((up / total) * 100, 2) if total else None
    return {
        "total": total,
        "up": up,
        "degraded": degraded,
        "down": down,
        "avg_latency_ms": avg_latency,
        "uptime_pct": uptime_pct,
    }


def last_check() -> dict | None:
    """El chequeo mas reciente (estado actual del MCP)."""
    sql = """
        SELECT id, checked_at, status, latency_ms, http_status, error
        FROM mcp_health_checks
        ORDER BY checked_at DESC
        LIMIT 1
    """
    rows = _rows(sql, [])
    return rows[0] if rows else None


def recent_outages(days: int = 7, limit: int = 100) -> list[dict]:
    """Historial de caidas: chequeos con status != 'up' en los ultimos N dias."""
    sql = """
        SELECT id, checked_at, status, latency_ms, http_status, error
        FROM mcp_health_checks
        WHERE status <> 'up'
          AND checked_at >= NOW() - (%s || ' days')::INTERVAL
        ORDER BY checked_at DESC
        LIMIT %s
    """
    return _rows(sql, [str(days), limit])
