"""Guard: la paleta de marca se declara UNA sola vez, en demo/tokens.css.

PROBLEMA QUE RESUELVE: landing.html y login.html traian su propia copia de la
paleta como literales hex y los verdes divergieron. tokens.css tenia
--wag-green-mid en #4a8a62 y la landing #3d7a54 para el mismo rol. No son
equivalentes: #4a8a62 da 3.88:1 sobre #faf8f2 y 4.11:1 bajo texto blanco,
contra 4.81:1 y 5.11:1 de #3d7a54, y la landing usa ese token justo donde AA
pide 4.5:1 (.section-label a 11px, .btn-cta-sm con color #fff). La divergencia
no era cosmetica: una de las dos copias no era accesible.

Fuera de alcance: los hex sueltos dentro de reglas (los gradientes de la
landing) no son declaraciones de token y este guard no los mira.
errors/500.html declara su propia paleta a proposito — handler500 corre sin
context processors y no puede usar {% static %} — pero es la del dashboard
(css/variables.css) y no intersecta con la de marca.
"""
from __future__ import annotations

import re
import unittest
from pathlib import Path

_APP = Path(__file__).resolve().parent.parent.parent
_TOKENS = _APP / "static" / "demo" / "tokens.css"

# consumidores: leen la paleta, no la copian
_CONSUMIDORES = ("templates/landing.html", "templates/login.html")

_DECL_WAG = re.compile(r"(--wag-[a-z0-9-]+)\s*:\s*(#[0-9a-fA-F]{3,8})")
_DECL_TOKEN = re.compile(r"(--[a-z0-9-]+)\s*:\s*(#[0-9a-fA-F]{3,8})")
# los roles de marca en los consumidores tienen que ser var(--wag-*): un hex
# aca es lo que dejo divergir a green-mid sin que ningun valor se repitiera
_ROL_DE_MARCA = re.compile(
    r"--(bg|bg-warm|chrome|ink|charcoal|slate|ash|stone|cream[a-z-]*"
    r"|green[a-z-]*|amber)\s*:\s*#"
)


def _archivos_servidos() -> list[Path]:
    candidatos = list(_APP.rglob("*.html")) + list(_APP.rglob("*.css"))
    return sorted(
        p
        for p in candidatos
        if p != _TOKENS and "node_modules" not in p.parts
    )


class PaletaUnicaEnTokensTests(unittest.TestCase):
    """Hereda de `unittest.TestCase` y no de `SimpleTestCase` a proposito:
    inspecciona archivos, no comportamiento de Django, asi que corre sin
    la BD ni el ORM: inspecciona archivos. Django igual crea la BD de test porque lo
    hace el DiscoverRunner. `manage.py test` y pytest lo recogen los dos.
    """

    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        cls.paleta = {
            hexa.lower(): nombre
            for nombre, hexa in _DECL_WAG.findall(
                _TOKENS.read_text(encoding="utf-8")
            )
        }

    def test_tokens_css_declara_la_paleta(self):
        self.assertGreaterEqual(
            len(self.paleta),
            12,
            "no se leyo la paleta --wag-* de tokens.css: si la renombraste, "
            "actualiza _DECL_WAG o este guard queda vacio y no protege nada",
        )

    def test_ningun_archivo_redeclara_un_hex_de_la_paleta(self):
        for archivo in _archivos_servidos():
            rel = archivo.relative_to(_APP).as_posix()
            texto = archivo.read_text(encoding="utf-8", errors="replace")
            for nombre, hexa in _DECL_TOKEN.findall(texto):
                if nombre.startswith("--wag-"):
                    continue
                canonico = self.paleta.get(hexa.lower())
                if canonico is None:
                    continue
                with self.subTest(archivo=rel, token=nombre):
                    self.fail(
                        f"{rel} declara `{nombre}: {hexa}`, que ya es "
                        f"`{canonico}` en demo/tokens.css. La paleta se declara "
                        f"una sola vez: usa `{nombre}: var({canonico})`"
                    )

    def test_los_consumidores_no_declaran_roles_de_marca_con_hex(self):
        for rel in _CONSUMIDORES:
            texto = (_APP / rel).read_text(encoding="utf-8")
            for match in _ROL_DE_MARCA.finditer(texto):
                with self.subTest(archivo=rel, decl=match.group(0)):
                    self.fail(
                        f"{rel} declara `{match.group(0).strip()}...` con un hex "
                        "propio. Un valor distinto al de tokens.css no se "
                        "detecta comparando valores: tiene que ser var(--wag-*)"
                    )

    def test_los_consumidores_cargan_tokens_css(self):
        for rel in _CONSUMIDORES:
            with self.subTest(archivo=rel):
                self.assertIn(
                    "demo/tokens.css",
                    (_APP / rel).read_text(encoding="utf-8"),
                    f"{rel} referencia var(--wag-*) pero no carga tokens.css: "
                    "las variables quedan sin resolver y el color se cae",
                )
