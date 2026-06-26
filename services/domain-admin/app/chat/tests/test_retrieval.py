"""Tests del retrieval service (HU-49.2)."""
from __future__ import annotations

from unittest.mock import MagicMock, patch

import pytest

from chat.retrieval import (
    MIN_SCORE,
    RetrievalService,
    _LexicalScorer,
    _cosine,
    _format_chunk,
    _tokenize,
)
from chat.models import RagContext, Source


class FakeEmbedder:
    """Embedder fake: cosine 1.0 si los textos comparten tokens, 0 si no."""

    def __init__(self, mode: str = "lexical") -> None:
        self.mode = mode
        self.calls: list[list[str]] = []

    def embed(self, texts: list[str]) -> list[list[float]]:
        self.calls.append(texts)
        if self.mode == "zero":
            return [[0.0] * 8 for _ in texts]
        if self.mode == "lexical":
            return [_lexical_vec(t) for t in texts]
        if self.mode == "exact_match":
            dim = 16
            q_vec = [0.0] * dim
            q_vec[0] = 1.0
            out = [q_vec]
            for t in texts[1:]:
                v = [0.0] * dim
                if t == texts[0]:
                    v[0] = 1.0
                out.append(v)
            return out
        raise ValueError(f"mode desconocido: {self.mode}")


def _lexical_vec(text: str) -> list[float]:
    """Vector toy: bag-of-words hashed a 16 dims."""
    vec = [0.0] * 16
    for w in _tokenize(text):
        h = hash(w) % 16
        vec[h] += 1.0
    n = sum(x * x for x in vec) ** 0.5
    return [x / n for x in vec] if n else vec


@pytest.fixture
def mock_rows():
    return [
        ("agent", "uuid-1", "Bot de Soporte", "Agent: Bot de Soporte | slug=soporte-bot | description=atiende tickets"),
        ("skill", "uuid-2", "Send Email", "Skill: Send Email | description=envia emails transaccionales"),
        ("project", "uuid-3", "API Gateway", "Project: API Gateway | slug=api-gateway | description=puerta de entrada"),
    ]


def test_retrieve_sin_rows_devuelve_empty():
    with patch("chat.retrieval._fetch_source_rows", return_value=[]):
        result = RetrievalService().retrieve("cualquier cosa")
    assert result.is_empty
    assert result.sources == []


def test_retrieve_sin_embedder_usa_scorer_lexical(mock_rows):
    with patch("chat.retrieval._fetch_source_rows", return_value=mock_rows):
        result = RetrievalService().retrieve("soporte bot")
    assert not result.is_empty
    assert len(result.sources) == 1
    assert result.sources[0].table == "agent"
    assert result.sources[0].title == "Bot de Soporte"


def test_retrieve_sin_embedder_score_bajo_usa_summary_fallback(mock_rows):
    """Cuando el score no llega al threshold, devolvemos 1 chunk diverso por tabla.

    Esto es la mejora del summary fallback: si la query es muy abstracta
    y no matchea con nada especifico, devolvemos un muestreo de las tablas
    principales para que el LLM tenga contexto amplio.
    """
    with patch("chat.retrieval._fetch_source_rows", return_value=mock_rows):
        result = RetrievalService().retrieve("xyzzy palabra_inexistente")
    assert not result.is_empty
    assert len(result.sources) > 0
    tables = {s.table for s in result.sources}
    assert "agent" in tables
    assert "skill" in tables


def test_retrieve_con_embedder_exact_match():
    rows = [
        ("agent", "uuid-1", "Bot de Soporte", "Bot de Soporte"),
        ("skill", "uuid-2", "Send Email", "Send Email"),
    ]
    embedder = FakeEmbedder(mode="exact_match")
    with patch("chat.retrieval._fetch_source_rows", return_value=rows):
        result = RetrievalService(embedding_provider=embedder).retrieve("Bot de Soporte")
    assert not result.is_empty
    assert any(s.title == "Bot de Soporte" for s in result.sources)
    assert result.sources[0].score == pytest.approx(1.0)


def test_retrieve_con_embedder_cero_usa_summary_fallback(mock_rows):
    """Embedder que retorna vectores cero: el score siempre es 0, se activa summary fallback."""
    embedder = FakeEmbedder(mode="zero")
    with patch("chat.retrieval._fetch_source_rows", return_value=mock_rows):
        result = RetrievalService(embedding_provider=embedder).retrieve("cualquier cosa")
    assert not result.is_empty
    assert len(result.sources) > 0


def test_retrieve_embedder_falla_fallback_lexico(mock_rows):
    class BadEmbedder:
        def embed(self, texts):
            raise RuntimeError("api caida")

    with patch("chat.retrieval._fetch_source_rows", return_value=mock_rows):
        result = RetrievalService(embedding_provider=BadEmbedder()).retrieve("soporte")
    assert not result.is_empty
    assert result.sources[0].title == "Bot de Soporte"


def test_retrieve_pregunta_vacia_devuelve_empty():
    result = RetrievalService().retrieve("   ")
    assert result.is_empty


def test_cosine_calculo_basico():
    assert _cosine([1, 0], [1, 0]) == pytest.approx(1.0)
    assert _cosine([1, 0], [0, 1]) == pytest.approx(0.0)
    assert _cosine([1, 1], [1, 1]) == pytest.approx(1.0)


def test_cosine_con_vectores_vacios():
    assert _cosine([], [1, 0]) == 0.0
    assert _cosine([1, 0], []) == 0.0
    assert _cosine([0, 0], [0, 0]) == 0.0


def test_format_chunk_incluye_campos_clave():
    row = {
        "name": "Test",
        "slug": "test-slug",
        "description": "una desc",
        "provider": "anthropic",
        "model": "claude-haiku",
    }
    text = _format_chunk("agent", row)
    assert "Test" in text
    assert "test-slug" in text
    assert "anthropic" in text


def test_tokenize_ignora_stopwords_y_puntuacion():
    assert "el" not in _tokenize("El agente es bueno")
    assert {"agente", "bueno"} == _tokenize("El agente es bueno")


def test_lexical_scorer_overlap_completo():
    scorer = _LexicalScorer()
    assert scorer.score({"alpha", "beta"}, "alpha beta gamma") == 1.0
    assert scorer.score({"alpha", "beta", "gamma"}, "alpha beta") == pytest.approx(2 / 3)


def test_top_score_con_sin_sources():
    assert RagContext().top_score == 0.0
    assert RagContext(sources=[Source(table="x", id="1", title="t", snippet="s", score=0.5)]).top_score == 0.5


def test_retrieve_respects_top_k(mock_rows):
    many_rows = [
        (f"agent", f"uuid-{i}", f"Agente {i}", f"Agent: Agente {i} | description=item comun")
        for i in range(20)
    ]
    with patch("chat.retrieval._fetch_source_rows", return_value=many_rows):
        result = RetrievalService().retrieve("agente comun item")
    assert len(result.sources) <= 10


def test_source_url_se_incluye(mock_rows):
    with patch("chat.retrieval._fetch_source_rows", return_value=mock_rows):
        result = RetrievalService().retrieve("soporte bot")
    assert "/agentes/detalle?id=uuid-1" == result.sources[0].url


def test_is_general_query_detecta_patrones():
    from chat.retrieval import _is_general_query
    assert _is_general_query("cuantos proyectos tienes?")
    assert _is_general_query("lista todos los agentes")
    assert _is_general_query("dame un resumen del sistema")
    assert _is_general_query("describe el sistema")
    assert _is_general_query("que proyectos tienes")
    assert _is_general_query("que agentes hay")
    assert _is_general_query("que skills existen")
    assert _is_general_query("cuales son los proyectos")
    assert _is_general_query("podrias buscar dentro de tus proyectos que habilidades tienen")
    assert not _is_general_query("que hace el bot de soporte")
    assert not _is_general_query("que proyectos usan python")
    assert not _is_general_query("hola")
    assert not _is_general_query("a")
    assert not _is_general_query("que pasa")


def test_detect_target_table_encontrla_tabla():
    from chat.retrieval import _detect_target_table
    assert _detect_target_table("cuantos proyectos tienes?") == "project"
    assert _detect_target_table("lista los agentes") == "agent"
    assert _detect_target_table("que skills hay?") == "skill"
    assert _detect_target_table("dame un resumen") is None
    assert _detect_target_table("que hace el bot") is None


def test_general_aggregate_cuenta_proyectos():
    """Una pregunta general con target=project devuelve conteo de proyectos."""
    from chat.retrieval import _AGGREGATE_SQL

    mock_result = (5, 4, "Api Gateway (api-gateway), Web App (web-app), Bot (bot)")

    with patch("chat.retrieval.connection.cursor") as mock_cursor_cls:
        cur = MagicMock()
        cur.fetchone.return_value = mock_result
        mock_cursor_cls.return_value.__enter__.return_value = cur
        result = RetrievalService().retrieve("cuantos proyectos tienes?")

    assert not result.is_empty
    assert any("PROJECT" in c.upper() for c in result.chunks)
    assert any("total=5" in c for c in result.chunks)