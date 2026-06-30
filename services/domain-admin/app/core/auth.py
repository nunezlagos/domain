"""Helpers de autenticacion/transporte, extraidos del patron duplicado
`_require_auth` / `_is_ajax` que vivia en cada app.

Contrato (identico a users.views._require_auth):
- require_auth(request): si NO hay sesion autenticada devuelve un
  HttpResponseRedirect("/login/"); si hay sesion devuelve None.
- is_ajax(request): True cuando el front pidio via fetch
  (header X-Requested-With == "fetch"), que es lo que dispara las ramas
  AJAX (re-render de partial / redirect para reload) en los mantenedores.

Uso tipico en una view::

    def my_view(request):
        if (redir := require_auth(request)):
            return redir
        ...
"""
from __future__ import annotations

from django.http import HttpResponse, HttpResponseRedirect

LOGIN_URL = "/login/"


def require_auth(request) -> HttpResponse | None:
    """Devuelve un redirect a /login/ si la sesion no esta autenticada, si no None."""
    if not request.session.get("authenticated"):
        return HttpResponseRedirect(LOGIN_URL)
    return None


def is_ajax(request) -> bool:
    """True si el request vino de un fetch del front (X-Requested-With: fetch)."""
    return request.headers.get("X-Requested-With") == "fetch"
