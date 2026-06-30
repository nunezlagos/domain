from __future__ import annotations

from django.shortcuts import render

from core.auth import require_auth

from . import services


def dashboard(request):
    if redir := require_auth(request):
        return redir

    uptime_24h = services.uptime_window(hours=24)
    uptime_7d = services.uptime_window(hours=24 * 7)
    current = services.last_check()
    outages = services.recent_outages(days=7)

    return render(request, "mcpuptime/list.html", {
        "uptime_24h": uptime_24h,
        "uptime_7d":  uptime_7d,
        "current":    current,
        "outages":    outages,
    })
