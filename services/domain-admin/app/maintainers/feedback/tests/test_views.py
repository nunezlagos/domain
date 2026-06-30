"""Tests de los views del feedback loop (HU-52.1).

Unitarios con mocks (RequestFactory + view functions directos), no tocan la
DB real — mismo patron que chat/tests/test_views.py.
"""
from __future__ import annotations

import json
from datetime import datetime, timezone
from unittest.mock import MagicMock, patch

import pytest
from django.test import RequestFactory

from maintainers.feedback import services
from maintainers.feedback import views as fb_views
from maintainers.feedback.rate_limit import reset_feedback


@pytest.fixture(autouse=True)
def _clear_rate_limit():
    reset_feedback()
    yield
    reset_feedback()


@pytest.fixture
def factory():
    return RequestFactory()


def _post(factory, body):
    request = factory.post(
        "/feedback/api/submit",
        data=json.dumps(body),
        content_type="application/json",
    )
    request.session = {"authenticated": True, "email": "admin@admin.com"}
    return request


# ── submit: auth ──────────────────────────────────────────────────────────

def test_submit_sin_auth_redirige(factory):
    request = factory.post(
        "/feedback/api/submit",
        data='{"message_id": 1, "rating": 1}',
        content_type="application/json",
    )
    request.session = {}
    response = fb_views.submit(request)
    assert response.status_code == 302
    assert "/login/" in response.url


# ── submit: validacion ────────────────────────────────────────────────────

def test_submit_body_invalido_400(factory):
    request = factory.post("/feedback/api/submit", data="", content_type="application/json")
    request.session = {"authenticated": True, "email": "admin@admin.com"}
    response = fb_views.submit(request)
    assert response.status_code == 400
    assert json.loads(response.content)["error"] == "body_invalido"


@patch("maintainers.feedback.views.services.submit_feedback")
def test_submit_rating_invalido_400(mock_submit, factory):
    mock_submit.side_effect = services.InvalidRatingError("rating debe ser 1 o -1")
    request = _post(factory, {"message_id": 5, "rating": 7})
    response = fb_views.submit(request)
    assert response.status_code == 400
    assert json.loads(response.content)["error"] == "invalid_rating"


@patch("maintainers.feedback.views.services.submit_feedback")
def test_submit_message_invalido_400(mock_submit, factory):
    mock_submit.side_effect = services.InvalidMessageError("message_id requerido")
    request = _post(factory, {"message_id": 0, "rating": 1})
    response = fb_views.submit(request)
    assert response.status_code == 400
    assert json.loads(response.content)["error"] == "invalid_message"


# ── submit: happy path ────────────────────────────────────────────────────

@patch("maintainers.feedback.views.services.submit_feedback")
def test_submit_rating_up_ok(mock_submit, factory):
    fb = MagicMock()
    fb.id = "uuid-1"
    fb.message_id = 42
    fb.rating = 1
    fb.skill_slug = "deploy"
    mock_submit.return_value = fb

    request = _post(factory, {"message_id": 42, "rating": 1, "skill_slug": "deploy"})
    response = fb_views.submit(request)

    assert response.status_code == 200
    data = json.loads(response.content)["data"]
    assert data["message_id"] == 42
    assert data["rating"] == 1
    assert data["skill_slug"] == "deploy"
    # user_email se toma de la sesion, no del body.
    mock_submit.assert_called_once_with(
        message_id=42, rating=1, skill_slug="deploy", comment="", user_email="admin@admin.com"
    )


@patch("maintainers.feedback.views.services.submit_feedback")
def test_submit_rating_down_con_comentario(mock_submit, factory):
    fb = MagicMock()
    fb.id = "uuid-2"
    fb.message_id = 7
    fb.rating = -1
    fb.skill_slug = ""
    mock_submit.return_value = fb

    request = _post(factory, {"message_id": 7, "rating": -1, "comment": "respuesta incompleta"})
    response = fb_views.submit(request)

    assert response.status_code == 200
    assert json.loads(response.content)["data"]["rating"] == -1
    mock_submit.assert_called_once_with(
        message_id=7, rating=-1, skill_slug="", comment="respuesta incompleta", user_email="admin@admin.com"
    )


# ── submit: idempotencia (upsert via update_or_create) ────────────────────

@patch("maintainers.feedback.views.services.submit_feedback")
def test_submit_segundo_voto_actualiza(mock_submit, factory):
    """El segundo voto sobre el mismo mensaje vuelve a llamar al service
    (que hace upsert): cambiar 👍 -> 👎 devuelve 200, no error."""
    fb = MagicMock()
    fb.id = "uuid-3"
    fb.message_id = 99
    fb.rating = -1
    fb.skill_slug = ""
    mock_submit.return_value = fb

    request = _post(factory, {"message_id": 99, "rating": -1})
    response = fb_views.submit(request)
    assert response.status_code == 200
    assert json.loads(response.content)["data"]["rating"] == -1


# ── submit: rate limit ────────────────────────────────────────────────────

@patch("maintainers.feedback.views.services.submit_feedback")
def test_submit_rate_limit_429(mock_submit, factory):
    fb = MagicMock()
    fb.id = "x"; fb.message_id = 1; fb.rating = 1; fb.skill_slug = ""
    mock_submit.return_value = fb

    last = None
    for i in range(31):  # limite 30/min
        request = _post(factory, {"message_id": i + 1, "rating": 1})
        last = fb_views.submit(request)
    assert last.status_code == 429
    assert json.loads(last.content)["error"] == "rate_limited"


# ── admin_list ────────────────────────────────────────────────────────────

def test_admin_list_sin_auth_redirige(factory):
    request = factory.get("/feedback/")
    request.session = {}
    response = fb_views.admin_list(request)
    assert response.status_code == 302


@patch("maintainers.feedback.views.render")
@patch("maintainers.feedback.views.services.aggregate_by_skill")
@patch("maintainers.feedback.views.services.list_feedback")
def test_admin_list_filtro_negativo(mock_list, mock_agg, mock_render, factory):
    mock_list.return_value = []
    mock_agg.return_value = []
    mock_render.return_value = MagicMock(status_code=200)

    request = factory.get("/feedback/?rating=negative")
    request.session = {"authenticated": True, "email": "admin@admin.com"}
    fb_views.admin_list(request)

    mock_list.assert_called_once_with(rating=services.SkillFeedback.RATING_DOWN)
    ctx = mock_render.call_args.args[2]
    assert ctx["rating_filter"] == "negative"


@patch("maintainers.feedback.views.render")
@patch("maintainers.feedback.views.services.aggregate_by_skill")
@patch("maintainers.feedback.views.services.list_feedback")
def test_admin_list_sin_filtro(mock_list, mock_agg, mock_render, factory):
    mock_list.return_value = []
    mock_agg.return_value = []
    mock_render.return_value = MagicMock(status_code=200)

    request = factory.get("/feedback/")
    request.session = {"authenticated": True, "email": "admin@admin.com"}
    fb_views.admin_list(request)

    mock_list.assert_called_once_with(rating=None)
    ctx = mock_render.call_args.args[2]
    assert ctx["rating_filter"] == "all"


# ── helpers ───────────────────────────────────────────────────────────────

def test_parse_body_invalido(factory):
    request = factory.post("/x", data="nope", content_type="application/json")
    assert fb_views._parse_body(request) is None


def test_parse_body_valido(factory):
    request = factory.post("/x", data='{"a": 1}', content_type="application/json")
    assert fb_views._parse_body(request) == {"a": 1}
