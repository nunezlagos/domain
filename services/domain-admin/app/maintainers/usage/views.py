from __future__ import annotations

from django.shortcuts import render

from core.auth import require_auth

from . import queries

_VALID_DAYS = {7, 30, 90}
_DEFAULT_DAYS = 30


def dashboard(request):
    if redir := require_auth(request):
        return redir

    try:
        days = int(request.GET.get("days", _DEFAULT_DAYS))
    except (ValueError, TypeError):
        days = _DEFAULT_DAYS
    if days not in _VALID_DAYS:
        days = _DEFAULT_DAYS

    kpis        = queries.kpis(days)
    by_project  = queries.by_project(days)
    by_client   = queries.by_client(days)
    by_model    = queries.by_model(days)
    recent      = queries.recent_prompts(days)

    return render(request, "usage/dashboard.html", {
        "days":       days,
        "kpis":       kpis,
        "by_project": by_project,
        "by_client":  by_client,
        "by_model":   by_model,
        "recent":     recent,
    })
