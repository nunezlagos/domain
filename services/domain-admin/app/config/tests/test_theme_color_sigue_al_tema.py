"""Guard del cromo del navegador de la landing (DOMAINSERV-205).

PROBLEMA QUE RESUELVE: la landing conserva el toggle de modo oscuro, pero el
`theme-color` estaba clavado en el crema del tema claro, asi que en modo oscuro
la barra del navegador quedaba crema sobre una pagina casi negra. Y el script
inline del head leia el tema SOLO de `localStorage`, asi que la primera visita
con el sistema en oscuro recibia el tema claro.

Los dos sintomas son del mismo defecto: el cromo y el tema no estaban atados.

POR QUE ESTE GUARD Y NO UN TEST DE BROWSER: el comportamiento vive en JS del
lado del cliente y el repo no tiene runner de browser. Lo que si se puede fijar
sin browser es el invariante que hace correcto a ese JS:

1. los dos hex que el JS le pasa al `theme-color` son EXACTAMENTE los tokens que
   pintan el fondo en cada tema, asi que no pueden divergir en silencio
2. el script inline consulta `prefers-color-scheme`
3. el pintado del cromo ocurre ANTES del return temprano por `#modeToggle`

DECISION QUE PUEDE SORPRENDER: el JS lleva los hex literales en vez de leer el
color ya calculado con `getComputedStyle`. `landing.css` declara una transicion
sobre `background`, asi que leer el fondo justo despues del toggle puede devolver
un valor intermedio de la animacion en lugar del final. Los hex literales son
deterministas; el precio es la posible deriva, y ese precio lo cubre el punto 1.
"""
from __future__ import annotations

import re
import unittest
from pathlib import Path


_APP = Path(__file__).resolve().parent.parent.parent
_LANDING_HTML = _APP / "templates" / "landing.html"
_LANDING_JS = _APP / "static" / "landing" / "landing.js"
_TOKENS_CSS = _APP / "static" / "demo" / "tokens.css"

# el fondo de cada tema: landing.css:2 usa --wag-cream-soft para --bg y
# landing.css:18 lo cambia a --wag-ink bajo html.dark
_TOKEN_CLARO = "--wag-cream-soft"
_TOKEN_OSCURO = "--wag-ink"


def _hex_de_token(css: str, token: str) -> str:
    match = re.search(rf"{re.escape(token)}\s*:\s*(#[0-9a-fA-F]{{3,8}})", css)
    if match is None:
        raise AssertionError(
            f"{token} no esta declarado en tokens.css; este guard compara contra "
            "el token, asi que sin el no puede verificar nada"
        )
    return match.group(1).lower()


class ThemeColorSigueAlTemaTests(unittest.TestCase):
    """El cromo del navegador acompania al tema, y el tema respeta al sistema.

    Hereda de `unittest.TestCase` y no de `SimpleTestCase` a proposito, igual que
    el guard de la landing minima: inspecciona archivos, no comportamiento de
    Django, asi que corre sin settings ni BD.
    """

    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        cls.html = _LANDING_HTML.read_text(encoding="utf-8")
        cls.js = _LANDING_JS.read_text(encoding="utf-8")
        cls.tokens = _TOKENS_CSS.read_text(encoding="utf-8")

    def test_el_js_usa_los_mismos_hex_que_los_tokens_del_fondo(self):
        esperados = {
            _hex_de_token(self.tokens, _TOKEN_CLARO),
            _hex_de_token(self.tokens, _TOKEN_OSCURO),
        }
        en_js = {h.lower() for h in re.findall(r"#[0-9a-fA-F]{3,8}", self.js)}
        self.assertEqual(
            en_js,
            esperados,
            "los hex que landing.js le pasa al theme-color no son los tokens que "
            f"pintan el fondo. Esperados {sorted(esperados)}, encontrados "
            f"{sorted(en_js)}. Si cambio un token, hay que cambiarlos aca: el "
            "cromo del navegador tiene que coincidir con la pagina.",
        )

    def test_el_script_inline_respeta_el_tema_del_sistema(self):
        self.assertIn(
            "prefers-color-scheme",
            self.html,
            "el script inline del head decide el tema sin consultar "
            "prefers-color-scheme, asi que la primera visita con el sistema en "
            "oscuro recibe el tema claro.",
        )

    def test_la_preferencia_guardada_gana_sobre_la_del_sistema(self):
        self.assertIn(
            "domain-theme",
            self.html,
            "el script inline no lee la preferencia guardada; si el sistema "
            "manda siempre, el toggle deja de tener efecto entre visitas.",
        )

    def test_el_cromo_se_pinta_aunque_no_exista_el_toggle(self):
        """La ventana se acota a la LLAMADA, no al string `theme-color`.

        Primera version de este guard buscaba `theme-color` y pasaba con el
        invariante roto: ese string aparece en el `querySelector` de la
        declaracion, que igual queda antes del lookup del boton. El sabotaje lo
        destapo. Es el mismo falso verde que ya aparecio dos veces en este repo
        (DOMAINSERV-190 y DOMAINSERV-182): un guard de fuente tiene que mirar el
        fragmento que gobierna el comportamiento, no el bloque que lo contiene.
        """
        declaracion = self.js.find("var pintarCromo")
        self.assertNotEqual(
            declaracion, -1, "landing.js no define pintarCromo: el cromo no se pinta"
        )
        lookup_boton = self.js.find("getElementById('modeToggle')")
        self.assertNotEqual(
            lookup_boton, -1, "landing.js ya no busca #modeToggle; revisar este guard"
        )
        primera_llamada = self.js.find("pintarCromo(", declaracion + len("var pintarCromo"))
        self.assertNotEqual(
            primera_llamada, -1, "pintarCromo esta definida pero nunca se invoca"
        )
        self.assertLess(
            primera_llamada,
            lookup_boton,
            "la primera llamada a pintarCromo ocurre despues de buscar "
            "#modeToggle, y ese lookup tiene un return temprano: si el boton no "
            "esta, el cromo se queda con el hex del template.",
        )
