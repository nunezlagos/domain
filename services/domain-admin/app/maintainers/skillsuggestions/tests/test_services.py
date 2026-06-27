"""Tests del service de skillsuggestions (HU-52.3).

Unitarios: el ORM y el cliente HTTP al domain-mcp estan mockeados. Cubren:
  - extract_diff por kind (refine/split/merge/archive) + fallback defensivo.
  - el cliente _post: degradacion sin token, parseo OK, mapeo de error HTTP,
    y caida de conexion.
"""
from __future__ import annotations

import io
import json
import urllib.error
from unittest.mock import MagicMock, patch

import pytest

from maintainers.skillsuggestions import services
from maintainers.skillsuggestions.models import SkillSuggestion


def _sug(kind, payload):
    s = SkillSuggestion(skill_slug="deploy", kind=kind, payload=payload)
    return s


# ── extract_diff ─────────────────────────────────────────────────────────────
def test_extract_diff_refine():
    s = _sug("refine", {"current_content": "viejo", "new_content": "nuevo"})
    d = services.extract_diff(s)
    assert d["kind"] == "refine"
    assert d["before"] == "viejo"
    assert d["after"] == "nuevo"
    assert "viejo" in d["raw"]


def test_extract_diff_split_cuenta_hijos():
    s = _sug("split", {"children": [{"slug": "a"}, {"slug": "b"}, "ignorado"]})
    d = services.extract_diff(s)
    assert len(d["children"]) == 2  # el string no-dict se descarta
    assert "2" in d["summary"]


def test_extract_diff_merge_targets():
    s = _sug("merge", {"targets": ["x", "y"], "merged_content": "fusion"})
    d = services.extract_diff(s)
    assert d["merge"]["targets"] == ["x", "y"]
    assert d["merge"]["merged_content"] == "fusion"


def test_extract_diff_archive_reason():
    s = _sug("archive", {"reason": "sin uso 90d"})
    d = services.extract_diff(s)
    assert d["reason"] == "sin uso 90d"


def test_extract_diff_payload_no_dict_no_rompe():
    s = _sug("refine", ["lista", "no", "dict"])
    d = services.extract_diff(s)
    assert d["before"] is None and d["after"] is None  # defensivo
    assert d["raw"]  # el crudo se serializa igual


# ── cliente _post / transiciones ─────────────────────────────────────────────
def test_post_sin_token_degrada():
    with patch.object(services, "_token", return_value=""):
        with pytest.raises(services.ApiNotConfiguredError):
            services.approve("11111111-1111-1111-1111-111111111111")


def test_post_ok_devuelve_data():
    resp = io.BytesIO(json.dumps({"data": {"id": "abc", "status": "approved"}}).encode())
    resp.__enter__ = lambda *a: resp
    resp.__exit__ = lambda *a: False
    with patch("urllib.request.urlopen", return_value=resp):
        out = services.approve("11111111-1111-1111-1111-111111111111")
    assert out["status"] == "approved"


def test_post_http_error_mapea_code_y_status():
    err = urllib.error.HTTPError(
        url="x", code=409, msg="conflict", hdrs=None,
        fp=io.BytesIO(json.dumps({"error": "not_pending", "message": "ya revisada"}).encode()),
    )
    with patch("urllib.request.urlopen", side_effect=err):
        with pytest.raises(services.SuggestionApiError) as ei:
            services.reject("11111111-1111-1111-1111-111111111111")
    assert ei.value.status == 409
    assert ei.value.code == "not_pending"


def test_post_conexion_caida():
    with patch("urllib.request.urlopen", side_effect=urllib.error.URLError("down")):
        with pytest.raises(services.SuggestionApiError) as ei:
            services.apply("11111111-1111-1111-1111-111111111111")
    assert ei.value.status is None


def test_count_pending_filtra_pending():
    with patch.object(services.SkillSuggestion, "objects") as mgr:
        mgr.filter.return_value.count.return_value = 7
        assert services.count_pending() == 7
        mgr.filter.assert_called_once_with(status=SkillSuggestion.STATUS_PENDING)
