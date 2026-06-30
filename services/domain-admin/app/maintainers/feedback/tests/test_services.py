"""Tests del service del feedback (HU-52.1).

Unitarios con mocks del ORM (update_or_create) — no tocan la DB real.
"""
from __future__ import annotations

from unittest.mock import MagicMock, patch

import pytest

from maintainers.feedback import services


def test_rating_invalido_lanza_error():
    with pytest.raises(services.InvalidRatingError):
        services.submit_feedback(message_id=1, rating=0)
    with pytest.raises(services.InvalidRatingError):
        services.submit_feedback(message_id=1, rating=2)


def test_message_id_invalido_lanza_error():
    with pytest.raises(services.InvalidMessageError):
        services.submit_feedback(message_id=0, rating=1)
    with pytest.raises(services.InvalidMessageError):
        services.submit_feedback(message_id=-5, rating=1)
    with pytest.raises(services.InvalidMessageError):
        services.submit_feedback(message_id="abc", rating=1)


@patch("maintainers.feedback.services.SkillFeedback")
def test_submit_normaliza_strings_vacios_a_none(mock_model):
    fb = MagicMock()
    mock_model.objects.update_or_create.return_value = (fb, True)
    mock_model.RATING_UP = 1
    mock_model.RATING_DOWN = -1

    services.submit_feedback(
        message_id=10, rating=1, skill_slug="   ", comment="", user_email="  "
    )
    kwargs = mock_model.objects.update_or_create.call_args.kwargs
    assert kwargs["message_id"] == 10
    defaults = kwargs["defaults"]
    assert defaults["skill_slug"] is None
    assert defaults["comment"] is None
    assert defaults["user_email"] is None
    assert defaults["rating"] == 1


@patch("maintainers.feedback.services.SkillFeedback")
def test_submit_upsert_por_message_id(mock_model):
    """update_or_create con clave message_id => idempotente (1 por mensaje)."""
    fb = MagicMock()
    mock_model.objects.update_or_create.return_value = (fb, False)
    mock_model.RATING_UP = 1
    mock_model.RATING_DOWN = -1

    result = services.submit_feedback(
        message_id=42, rating=-1, skill_slug="deploy", comment="malo", user_email="a@b.com"
    )
    assert result is fb
    kwargs = mock_model.objects.update_or_create.call_args.kwargs
    assert kwargs["message_id"] == 42
    assert kwargs["defaults"]["rating"] == -1
    assert kwargs["defaults"]["skill_slug"] == "deploy"
    assert kwargs["defaults"]["comment"] == "malo"
