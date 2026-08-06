"""Guard: el Caddyfile conserva el bloque que sirve el panel sobre TLS.

PROBLEMA QUE RESUELVE: hasta DOMAINSERV-204 el UNICO bloque que servia el panel era el
catch-all `:80`, o sea que la cookie de sesion viajaba en claro. Con la SECRET_KEY rotada ya
no se puede forjar una sesion offline, pero capturar la que viaja por una red hostil y
reusarla tal cual sigue alcanzando.

Este guard existe porque el bloque de TLS es facil de perder sin que nada falle: el panel
sigue respondiendo por el `:80`, asi que el sintoma no es una caida sino una degradacion
silenciosa a http. Es el modo de falla que documenta la policy `guards-deben-ejecutarse`.

NO verifica que haya TLS en produccion — eso depende de un registro DNS que todavia no
existe y se verifica contra el ambiente real, no desde un test.
"""
from __future__ import annotations

import re
import unittest
from pathlib import Path

# parents[4] es services/: el test vive en services/domain-admin/app/config/tests/
_CADDYFILE = Path(__file__).resolve().parents[4] / "caddy" / "Caddyfile"


class CaddyfileSirveElPanelPorTlsTests(unittest.TestCase):
    def setUp(self):
        # falla en vez de skipear: un guard que se saltea en silencio no es un guard, y
        # ademas no hay workflow de CI para domain-admin, asi que un skip no lo veria NADIE
        self.assertTrue(
            _CADDYFILE.is_file(),
            f"no encontre el Caddyfile en {_CADDYFILE}. Si la ruta cambio, actualizala: si "
            "esto se convierte en skip, el guard deja de proteger el bloque de TLS",
        )
        self.fuente = _CADDYFILE.read_text(encoding="utf-8")

    def test_existe_un_bloque_de_panel_parametrizado_por_admin_domain(self):
        self.assertIn(
            "{$ADMIN_DOMAIN", self.fuente,
            "se perdio el bloque del panel por dominio: sin el, la unica via al panel es el "
            "catch-all :80 y la cookie de sesion vuelve a viajar en claro",
        )
        self.assertRegex(
            self.fuente, r"\{\$ADMIN_DOMAIN:[^}]+\}",
            "el placeholder tiene que traer default: `{$ADMIN_DOMAIN}` vacio deja un bloque "
            "sin direccion de sitio y Caddy no arranca en un ambiente sin dominio",
        )

    def test_el_bloque_del_panel_reversea_al_container_del_admin(self):
        bloque = self._bloque_de_admin_domain()
        self.assertIn(
            "reverse_proxy domain-admin:80", bloque,
            "el bloque de dominio tiene que llevar al panel; si no, sirve otra cosa por TLS "
            "y el panel sigue solo en http",
        )

    def test_el_bloque_del_panel_tiene_crowdsec_y_las_cabeceras(self):
        """Un bloque nuevo que se saltea el filtrado es una puerta sin guardia.

        El `:80` pasa por crowdsec y por las cabeceras de seguridad. Si el bloque de TLS no
        lo hace, mover el trafico a TLS EMPEORA la postura: se gana confidencialidad y se
        pierde el baneo de IPs y el X-Frame-Options.
        """
        bloque = self._bloque_de_admin_domain()
        self.assertIn("crowdsec", bloque,
                      "el bloque de dominio no pasa por crowdsec: las IPs baneadas entrarian")
        self.assertIn("import cabeceras_de_seguridad", bloque,
                      "el bloque de dominio no importa las cabeceras de seguridad")

    def test_las_cabeceras_son_un_snippet_compartido(self):
        """Duplicarlas es lo que hace que los dos bloques se desalineen con el tiempo."""
        self.assertIn("(cabeceras_de_seguridad)", self.fuente)
        self.assertEqual(
            2, self.fuente.count("import cabeceras_de_seguridad"),
            "los DOS bloques que sirven el panel (dominio y :80) tienen que importar las "
            "cabeceras: el que quede sin ellas es el que un atacante va a usar",
        )

    def test_el_bloque_del_panel_va_antes_del_catch_all(self):
        """El `:80` es catch-all y el orden de escritura decide: si va primero, se come todo."""
        pos_dominio = self.fuente.index("{$ADMIN_DOMAIN")
        pos_catch_all = self.fuente.index("\n:80 {")
        self.assertLess(
            pos_dominio, pos_catch_all,
            "el bloque de dominio quedo DESPUES del catch-all :80, que lo hace inalcanzable: "
            "el mismo orden que el Caddyfile ya respeta para quiensabe.cl",
        )

    def _bloque_de_admin_domain(self) -> str:
        match = re.search(r"\{\$ADMIN_DOMAIN:[^}]+\}\s*\{(.*?)\n\}", self.fuente, re.DOTALL)
        self.assertIsNotNone(match, "no pude aislar el bloque de {$ADMIN_DOMAIN}")
        return match.group(1)
