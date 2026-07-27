"""Guard contra metricas inventadas en la landing publica (DOMAINSERV-169).

PROBLEMA QUE RESUELVE: `landing.html` se sirve en `/` sin autenticacion, y hasta
HEAD 27a26b58 publicaba cuatro cifras que nadie medía — "247 skills activas"
(el catalogo real tiene 18), "1.247 calls" y "892 calls" para skills que no
existen en el catalogo, y "68% success" para otra inexistente. Una cifra
inventada en una pagina publica es indistinguible de una medicion real: nadie
que la lea puede saber que es falsa.

No hay nada en el pipeline que lo detecte. La landing es un template estatico,
no pasa por ningun form ni serializer, y `test_views.py` solo cubre los
mantenedores. Este guard cierra ese hueco por inspeccion del template.

Los tres primeros tests prohiben cifras donde la UI las presenta como metrica
(los badges y las tarjetas flotantes del hero). El cuarto prohibe PROMETER
metricas que hoy no llegan al usuario: `internal/service/skill_metrics` y
`internal/service/skill_ab_test` estan implementados en el MCP pero
DESCONECTADOS — nadie invoca `UpsertDaily` fuera de sus tests, y
`feedback_aggregator.go:13` lo dice explicito ("en HU-52.2 se integrara con
skill_metrics"). Ver DOMAINSERV-174.

Mantenimiento: al agregar una skill a las tarjetas de ejemplo, agregala tambien
a `SKILLS_DEL_CATALOGO`. Ese paso manual es intencional — obliga a verificar que
la skill exista antes de publicarla.
"""
from __future__ import annotations

import re
import unittest
from pathlib import Path


_LANDING_PATH = (
    Path(__file__).resolve().parent.parent.parent / "templates" / "landing.html"
)

# Skills verificadas en el catalogo del MCP. Las globales salen del seeder
# (`internal/seeds/skill_catalog.go`), asi que existen en cualquier instalacion;
# las de proyecto existen en la BD de domain-services.
SKILLS_DEL_CATALOGO = frozenset(
    {
        "commit-message",
        "diff-summarize",
        "error-classify",
        "extract-entities",
        "gherkin-from-bug",
        "intake-classify",
        "intake-structure",
        "judgment-day",
        "orca-webbrowser-workflow",
        "orca-worktree-workflow",
        "requesting-code-review",
        "schema-migration-authoring",
        "sql-explain-impact",
        "summarize",
        "text-redact-secrets",
        "wcag-audit",
        "mcp-tool-security-review",
        "vps-deploy-admin",
    }
)

# Terminos que describen mediciones que el usuario NO recibe hoy.
METRICAS_DESCONECTADAS = (
    "success rate",
    "latencia",
    "a/b testing",
)


def _leer_landing() -> str:
    return _LANDING_PATH.read_text(encoding="utf-8")


def _texto_visible(fragmento: str) -> str:
    """Devuelve solo el texto entre tags, descartando atributos.

    Los atributos traen digitos legitimos (`font-size:8px`, colores hex) que
    no son metricas y no deben disparar el guard.
    """
    return " ".join(re.findall(r">([^<>]+)<", fragmento))


class LandingSinMetricasInventadasTests(unittest.TestCase):
    """La landing publica no puede afirmar numeros que nadie mide.

    Hereda de `unittest.TestCase` y no de `SimpleTestCase` a proposito: el guard
    inspecciona un archivo de template, no comportamiento de Django, asi que
    corre sin settings ni BD. `manage.py test` lo recoge igual.
    """

    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        cls.html = _leer_landing()

    def test_los_badges_de_skill_no_publican_cifras(self):
        badges = re.findall(r'class="fmc-badge[^"]*"\s*>([^<]*)<', self.html)
        self.assertTrue(badges, "no se encontro ningun fmc-badge en la landing")
        for badge in badges:
            with self.subTest(badge=badge):
                self.assertFalse(
                    any(char.isdigit() for char in badge),
                    f'el badge "{badge.strip()}" publica una cifra: la UI la presenta '
                    "como metrica medida y nadie la calcula (DOMAINSERV-174)",
                )

    def test_las_tarjetas_del_hero_no_publican_cifras(self):
        tarjetas = re.findall(
            r'class="hero-card[^"]*">(.*?)</div>', self.html, re.DOTALL
        )
        self.assertTrue(tarjetas, "no se encontro ninguna hero-card en la landing")
        for tarjeta in tarjetas:
            texto = _texto_visible(tarjeta).strip()
            with self.subTest(tarjeta=texto):
                self.assertFalse(
                    any(char.isdigit() for char in texto),
                    f'la hero-card "{texto}" publica una cifra sin respaldo. Los '
                    "stats del hero (hero-stat-num) si son verificables y quedan "
                    "fuera de este guard",
                )

    def test_los_nombres_de_skill_existen_en_el_catalogo(self):
        nombres = re.findall(r'class="fmc-name"\s*>([^<]*)<', self.html)
        self.assertTrue(nombres, "no se encontro ningun fmc-name en la landing")
        for nombre in nombres:
            slug = nombre.strip()
            with self.subTest(skill=slug):
                self.assertIn(
                    slug,
                    SKILLS_DEL_CATALOGO,
                    f'la landing publica la skill "{slug}", que no existe en el '
                    "catalogo. Si la agregaste al seeder, agregala tambien a "
                    "SKILLS_DEL_CATALOGO",
                )

    def test_las_features_no_prometen_metricas_desconectadas(self):
        seccion = re.search(
            r'<section id="caracteristicas">(.*?)</section>', self.html, re.DOTALL
        )
        self.assertIsNotNone(
            seccion, "no se encontro la seccion #caracteristicas en la landing"
        )
        texto = _texto_visible(seccion.group(1)).lower()
        for termino in METRICAS_DESCONECTADAS:
            with self.subTest(termino=termino):
                self.assertNotIn(
                    termino,
                    texto,
                    f'la seccion de features promete "{termino}", que hoy no llega al '
                    "usuario: skill_metrics y skill_ab_test estan implementados pero "
                    "desconectados (DOMAINSERV-174). La seccion #por-que si puede "
                    "nombrarlos como problema",
                )
