"""Guard de la landing publica: minima, sin cifras y sin superficie (DOMAINSERV-169).

HISTORIA DE ESTE GUARD, que importa para no re-aflojarlo:

1. DOMAINSERV-169. `landing.html` se sirve en `/` sin autenticacion y publicaba
   cuatro cifras que nadie medía — "247 skills activas" (el catalogo real tiene
   18), "1.247 calls" y "892 calls" para skills inexistentes, y "68% success"
   para otra. Una cifra inventada en una pagina publica es indistinguible de una
   medicion real: nadie que la lea puede saber que es falsa. Nada en el pipeline
   lo detectaba, porque la landing es un template estatico que no pasa por
   ningun form ni serializer.

2. DOMAINSERV-174 sumo el cuarto check: prohibir PROMETER metricas que hoy no
   llegan al usuario. `internal/service/skill_metrics` y `skill_ab_test` estan
   implementados en el MCP pero DESCONECTADOS — nadie invoca `UpsertDaily`
   fuera de sus tests.

3. 2026-07-30. La landing se reduce a una puerta: logo, una linea y el boton de
   entrar. Los cuatro checks anteriores miraban `fmc-badge`, `hero-card`,
   `fmc-name` y la seccion `#features`, que dejaron de existir — habrian fallado
   por la razon equivocada. El guard pasa a vigilar la invariante nueva, que
   SUBSUME la vieja: en una pagina sin contenido no hay donde publicar una cifra.

Lo que se vigila ahora, y por que cada cosa:

- Sin cifras en el texto visible. Es el invariante de 169, ahora trivial de
  cumplir y facil de verificar sobre la pagina entera en vez de por seccion.
- Sin comandos de instalacion ni URLs internas. La revision de seguridad del
  2026-07-29 los marco como superficie de enumeracion gratuita en una pagina
  publica: `curl | sudo bash` desde `main` sin pinear, las formas de URL de
  `/mcp` y `/api/v1/`, y el path absoluto del `.env`.
- Sin promesas de metricas desconectadas. Es el check de 174, ahora sobre toda
  la pagina. El blocklist esta en espanol Y en ingles a proposito: cuando estaba
  solo en ingles, "tasa de acierto por skill" prometia `success_rate` y ningun
  guard lo veia.
- El boton de entrar existe y apunta al login. Sin el, la unica funcion de la
  pagina desaparece y no hay forma de llegar al portal.
- La pagina sigue siendo chica. Es un tope, no una metrica de calidad: si
  alguien vuelve a llenarla de secciones, este test falla y obliga a decidirlo
  a proposito en vez de que la superficie crezca de a poco.
"""
from __future__ import annotations

import re
import unittest
from pathlib import Path


_LANDING_PATH = (
    Path(__file__).resolve().parent.parent.parent / "templates" / "landing.html"
)

# Terminos que describen mediciones que el usuario NO recibe hoy.
# En los dos idiomas: la landing se escribe en espanol y un blocklist solo en
# ingles deja pasar la parafrasis.
METRICAS_DESCONECTADAS = (
    "success rate",
    "latencia",
    "a/b testing",
    "tasa de acierto",
    "tasa de exito",
    "tasa de éxito",
    "disponibilidad",
    "uptime",
)

# Comandos y rutas que no van en una pagina publica. Cada uno le ahorra un paso
# de reconocimiento a quien busque un blanco.
SUPERFICIE_PROHIBIDA = (
    "curl ",
    "sudo bash",
    "raw.githubusercontent",
    "/api/v1",
    "/opt/",
    "install.sh",
)

# Tope de tamano de la puerta. Generoso respecto de las ~55 lineas actuales:
# deja agregar un parrafo o un meta sin tocar el guard, y ataja que vuelvan las
# secciones.
MAX_LINEAS = 80


def _leer_landing() -> str:
    return _LANDING_PATH.read_text(encoding="utf-8")


def _texto_visible(html: str) -> str:
    """Devuelve el texto entre tags, sin atributos ni contenido de script/style.

    Los atributos traen digitos legitimos (`width="72"`, colores hex) que no son
    metricas. El contenido de `script` y `style` tambien, y encima es codigo: si
    manana el snippet de tema usa un numero, el guard no tiene que confundirlo
    con una cifra publicada.
    """
    sin_codigo = re.sub(
        r"<(script|style)\b[^>]*>.*?</\1>", " ", html, flags=re.DOTALL | re.IGNORECASE
    )
    return " ".join(re.findall(r">([^<>]+)<", sin_codigo))


class LandingMinimaTests(unittest.TestCase):
    """La landing publica es una puerta: sin cifras, sin superficie, con salida.

    Hereda de `unittest.TestCase` y no de `SimpleTestCase` a proposito: el guard
    inspecciona un archivo de template, no comportamiento de Django, asi que
    corre sin settings ni BD. `manage.py test` lo recoge igual.
    """

    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        cls.html = _leer_landing()
        cls.visible = _texto_visible(cls.html)

    def test_el_texto_visible_no_publica_cifras(self):
        digitos = [char for char in self.visible if char.isdigit()]
        self.assertEqual(
            digitos,
            [],
            "la landing publica una cifra en su texto visible: "
            f"{self.visible.strip()!r}. En una pagina publica una cifra es "
            "indistinguible de una medicion real (DOMAINSERV-169). Si de verdad "
            "hay algo que medir, tiene que salir del contexto de la vista, no "
            "del template.",
        )

    def test_no_publica_comandos_de_instalacion_ni_rutas_internas(self):
        bajo = self.html.lower()
        for fragmento in SUPERFICIE_PROHIBIDA:
            with self.subTest(fragmento=fragmento):
                self.assertNotIn(
                    fragmento,
                    bajo,
                    f'la landing publica "{fragmento}". Es una pagina sin '
                    "autenticar delante del login: cada comando o ruta interna "
                    "que muestre le ahorra reconocimiento a quien busque un "
                    "blanco. La instalacion se documenta en el repo, no aca.",
                )

    def test_no_promete_metricas_desconectadas(self):
        bajo = self.visible.lower()
        for termino in METRICAS_DESCONECTADAS:
            with self.subTest(termino=termino):
                self.assertNotIn(
                    termino,
                    bajo,
                    f'la landing promete "{termino}", que hoy no llega al '
                    "usuario: skill_metrics y skill_ab_test estan implementados "
                    "pero desconectados (DOMAINSERV-174).",
                )

    def test_el_boton_de_entrar_apunta_al_login(self):
        self.assertRegex(
            self.html,
            r"""\{%\s*url\s+['"]login['"]\s*%\}""",
            "la landing no tiene un enlace al login resuelto con {% url 'login' %}. "
            "Es la unica funcion de la pagina: sin el no hay forma de llegar al "
            "portal. Con {% url %} y no con /login/ hardcodeado, para que un "
            "cambio de ruta no lo rompa en silencio.",
        )

    def test_la_landing_sigue_siendo_minima(self):
        lineas = len(self.html.splitlines())
        self.assertLessEqual(
            lineas,
            MAX_LINEAS,
            f"la landing crecio a {lineas} lineas (tope {MAX_LINEAS}). Se redujo "
            "a una puerta a proposito, para no dejar expuesto el login detras de "
            "una pagina llena de informacion. Si hace falta agregar contenido, "
            "que sea una decision explicita: subi el tope en el mismo commit y "
            "deja el motivo.",
        )
