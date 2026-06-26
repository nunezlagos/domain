from __future__ import annotations

from django.shortcuts import render

from core.auth import require_auth

from . import queries

_VALID_DAYS = {7, 30, 90}
_DEFAULT_DAYS = 30
_VALID_BY = {"client", "user", "project"}
_DEFAULT_BY = "client"


def dashboard(request):
    if redir := require_auth(request):
        return redir

    try:
        days = int(request.GET.get("days", _DEFAULT_DAYS))
    except (ValueError, TypeError):
        days = _DEFAULT_DAYS
    if days not in _VALID_DAYS:
        days = _DEFAULT_DAYS

    by = request.GET.get("by", _DEFAULT_BY)
    if by not in _VALID_BY:
        by = _DEFAULT_BY

    user_filter = request.GET.get("user", "").strip()

    kpis        = queries.kpis(days)
    by_project  = queries.by_project(days)
    by_client   = queries.by_client(days)
    by_user     = queries.by_user(days)
    by_model    = queries.by_model(days)
    recent      = queries.recent_prompts(days, user_email=user_filter)

    return render(request, "usage/dashboard.html", {
        "days":         days,
        "by":           by,
        "user_filter":  user_filter,
        "kpis":         kpis,
        "by_project":   by_project,
        "by_client":    by_client,
        "by_user":      by_user,
        "by_model":     by_model,
        "recent":       recent,
    })
