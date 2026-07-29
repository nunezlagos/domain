"""Guard contra templates huerfanos en domain-admin.

PROBLEMA QUE RESUELVE: nada en el pipeline detecta un template que dejo de
usarse. `templates/_base.html` era la base del login hasta que d5ff738b lo
rediseño standalone, y `components/_sdd_icon.html` era el icono por fase del
flujo SDD hasta que 55a6f20a paso a Font Awesome. Los dos quedaron en el arbol
sin un solo referente, y el alcance del ticket que los perseguia caduco tres
veces porque la verificacion era manual.

El guard reconstruye la alcanzabilidad de cada template desde tres fuentes y
falla si sobra alguno:

1. Nombre literal en cualquier .py, tipico de render(request, <nombre>).
2. La convencion CRUD de `core/views.py`: CrudView.tpl(name) arma el nombre
   como namespace + sufijo, asi que ningun `agents/list.html` aparece literal
   en el codigo. Se cruza cada namespace declarado con cada sufijo pedido.
3. El cierre transitivo de extends e include entre templates.

LIMITE CONOCIDO, en las dos direcciones. Solo ve nombres literales entre comillas
dobles, asi que (a) si algun dia un view arma el nombre por concatenacion fuera de
tpl(), este guard lo reporta como huerfano y la respuesta correcta es extender el
modelo, no silenciarlo; y (b) tampoco distingue un render real de una mencion del
nombre en un comentario, un docstring o un test: cualquier aparicion entre comillas
dobles en un .py de app/ alcanza para blanquear un template. Hoy landing.html queda
alcanzable tambien por config/tests/test_landing_sin_metricas_inventadas.py:34, asi
que si mañana se borrara su vista este guard seguiria verde.

DONDE CORRE: solo donde alguien ejecute la suite de domain-admin a mano. Ningun
workflow de .github/workflows/ corre tests de este servicio, asi que esto no es
proteccion automatica.
"""
from __future__ import annotations

import re
import unittest
from pathlib import Path

_APP = Path(__file__).resolve().parent.parent.parent
_TEMPLATES = _APP / "templates"

_RE_LITERAL = re.compile(r'"([A-Za-z0-9_./-]+\.html)"')
_RE_SUFIJO = re.compile(r'self\.tpl\(\s*"([^"]+)"')
_RE_NAMESPACE = re.compile(r'^\s*templates\s*=\s*"([a-z]+)"', re.MULTILINE)
_RE_HERENCIA = re.compile(r'\{%\s*(?:extends|include)\s+"([^"]+)"')


def _nombre(path: Path) -> str:
    return path.relative_to(_TEMPLATES).as_posix()


class TemplatesSinHuerfanosTests(unittest.TestCase):
    """Todo template del arbol tiene que ser alcanzable desde alguna vista.

    Hereda de `unittest.TestCase` y no de `SimpleTestCase` a proposito: inspecciona
    archivos, no comportamiento de Django, asi que corre sin settings ni BD.
    `manage.py test` lo recoge igual.
    """

    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        cls.todos = {_nombre(p) for p in _TEMPLATES.rglob("*.html")}
        codigo = "\n".join(
            p.read_text(encoding="utf-8") for p in _APP.rglob("*.py")
        )
        cls.sufijos = set(_RE_SUFIJO.findall(codigo))
        cls.namespaces = set(_RE_NAMESPACE.findall(codigo))
        cls.raices = set(_RE_LITERAL.findall(codigo)) | {
            f"{ns}/{sufijo}" for ns in cls.namespaces for sufijo in cls.sufijos
        }
        cls.herencia = {
            _nombre(p): set(_RE_HERENCIA.findall(p.read_text(encoding="utf-8")))
            for p in _TEMPLATES.rglob("*.html")
        }

    def test_el_modelo_de_alcanzabilidad_no_quedo_vacio(self):
        # sin esto, un regex roto haria pasar el guard reportando el arbol entero
        # como huerfano y el mensaje seria indistinguible de codigo muerto real
        self.assertTrue(
            self.sufijos, "no se detecto ningun self.tpl(...) en core/views.py"
        )
        self.assertTrue(
            self.namespaces, "no se detecto ningun namespace de templates en los CRUD"
        )
        self.assertTrue(
            any(self.herencia.values()),
            "no se detecto ningun extends/include en los templates",
        )

    def test_no_hay_templates_sin_referentes(self):
        alcanzables: set[str] = set()
        pendientes = [r for r in self.raices if r in self.todos]
        while pendientes:
            actual = pendientes.pop()
            if actual in alcanzables:
                continue
            alcanzables.add(actual)
            pendientes.extend(self.herencia.get(actual, ()))

        huerfanos = sorted(self.todos - alcanzables)
        self.assertEqual(
            huerfanos,
            [],
            f"templates sin un solo referente: {huerfanos}. Nadie los renderiza ni "
            "los extiende/incluye: borralos. Si el referente existe y este guard no "
            "lo ve, extende el modelo de alcanzabilidad del docstring",
        )
