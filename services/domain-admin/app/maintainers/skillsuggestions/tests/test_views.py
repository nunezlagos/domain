"""Tests de los views de skillsuggestions (HU-52.3).

Unitarios con RequestFactory + mocks (render/redirect/messages/services). No
tocan la DB real ni el domain-mcp. Verifican: guard de auth, default a pending,
filtros, 404 en detalle, y el mapeo de errores del Go a flash en la transicion.
"""
from __future__ import annotations

from unittest.mock import MagicMock, patch

import pytest
from django.http import Http404
from django.test import RequestFactory

from maintainers.skillsuggestions import services
from maintainers.skillsuggestions import views
from maintainers.skillsuggestions.models import SkillSuggestion

SID = "11111111-1111-1111-1111-111111111111"


@pytest.fixture
def factory():
    return RequestFactory()


def _auth(request):
    request.session = {"authenticated": True, "email": "admin@admin.com"}
    return request


# ── auth ─────────────────────────────────────────────────────────────────────
def test_list_sin_auth_redirige(factory):
    request = factory.get("/skill-suggestions/")
    request.session = {}
    resp = views.admin_list(request)
    assert resp.status_code == 302
    assert "/login/" in resp.url


def test_detail_sin_auth_redirige(factory):
    request = factory.get(f"/skill-suggestions/{SID}/")
    request.session = {}
    resp = views.detail(request, SID)
    assert resp.status_code == 302


# ── list: default + filtros ──────────────────────────────────────────────────
@patch("maintainers.skillsuggestions.views.render")
@patch("maintainers.skillsuggestions.views.services.kind_breakdown", return_value=[])
@patch("maintainers.skillsuggestions.views.services.count_pending", return_value=3)
@patch("maintainers.skillsuggestions.views.services.list_suggestions", return_value=[])
def test_list_default_pending(mock_list, *_):
    f = RequestFactory()
    request = _auth(f.get("/skill-suggestions/"))
    views.admin_list(request)
    # sin ?status => filtra pending
    assert mock_list.call_args.kwargs["status"] == SkillSuggestion.STATUS_PENDING


@patch("maintainers.skillsuggestions.views.render")
@patch("maintainers.skillsuggestions.views.services.kind_breakdown", return_value=[])
@patch("maintainers.skillsuggestions.views.services.count_pending", return_value=0)
@patch("maintainers.skillsuggestions.views.services.list_suggestions", return_value=[])
def test_list_status_all_sin_filtro(mock_list, *_):
    f = RequestFactory()
    request = _auth(f.get("/skill-suggestions/?status=all"))
    views.admin_list(request)
    assert mock_list.call_args.kwargs["status"] is None


@patch("maintainers.skillsuggestions.views.render")
@patch("maintainers.skillsuggestions.views.services.kind_breakdown", return_value=[])
@patch("maintainers.skillsuggestions.views.services.count_pending", return_value=0)
@patch("maintainers.skillsuggestions.views.services.list_suggestions", return_value=[])
def test_list_filtra_kind_y_skill(mock_list, *_):
    f = RequestFactory()
    request = _auth(f.get("/skill-suggestions/?status=approved&kind=refine&skill=deploy"))
    views.admin_list(request)
    kw = mock_list.call_args.kwargs
    assert kw["status"] == "approved"
    assert kw["kind"] == "refine"
    assert kw["skill_slug"] == "deploy"


# ── detail: 404 ──────────────────────────────────────────────────────────────
@patch("maintainers.skillsuggestions.views.services.get_suggestion", return_value=None)
def test_detail_no_existe_404(_):
    f = RequestFactory()
    request = _auth(f.get(f"/skill-suggestions/{SID}/"))
    with pytest.raises(Http404):
        views.detail(request, SID)


# ── transicion: happy + errores ──────────────────────────────────────────────
@patch("maintainers.skillsuggestions.views.redirect")
@patch("maintainers.skillsuggestions.views.messages")
@patch("maintainers.skillsuggestions.views.services.approve")
@patch("maintainers.skillsuggestions.views.services.get_suggestion")
def test_approve_ok_flash_success(mock_get, mock_approve, mock_msgs, mock_redirect):
    mock_get.return_value = MagicMock()
    f = RequestFactory()
    request = _auth(f.post(f"/skill-suggestions/{SID}/approve", {"comment": "ok"}))
    views.approve_view(request, SID)
    mock_approve.assert_called_once_with(SID)
    mock_msgs.success.assert_called_once()


@patch("maintainers.skillsuggestions.views.redirect")
@patch("maintainers.skillsuggestions.views.messages")
@patch("maintainers.skillsuggestions.views.services.approve")
@patch("maintainers.skillsuggestions.views.services.get_suggestion")
def test_approve_conflicto_mapea_mensaje(mock_get, mock_approve, mock_msgs, mock_redirect):
    mock_get.return_value = MagicMock()
    mock_approve.side_effect = services.SuggestionApiError("x", status=409, code="not_pending")
    f = RequestFactory()
    request = _auth(f.post(f"/skill-suggestions/{SID}/approve"))
    views.approve_view(request, SID)
    mock_msgs.error.assert_called_once()
    msg = mock_msgs.error.call_args.args[1]
    assert "pendiente" in msg.lower()


@patch("maintainers.skillsuggestions.views.redirect")
@patch("maintainers.skillsuggestions.views.messages")
@patch("maintainers.skillsuggestions.views.services.apply")
@patch("maintainers.skillsuggestions.views.services.get_suggestion")
def test_apply_no_configurado_flash_error(mock_get, mock_apply, mock_msgs, mock_redirect):
    mock_get.return_value = MagicMock()
    mock_apply.side_effect = services.ApiNotConfiguredError("falta token")
    f = RequestFactory()
    request = _auth(f.post(f"/skill-suggestions/{SID}/apply"))
    views.apply_view(request, SID)
    mock_msgs.error.assert_called_once()


@patch("maintainers.skillsuggestions.views.services.get_suggestion", return_value=None)
def test_transicion_404_si_no_existe(_):
    f = RequestFactory()
    request = _auth(f.post(f"/skill-suggestions/{SID}/reject"))
    with pytest.raises(Http404):
        views.reject_view(request, SID)
