"""Guard: los flags de cookie Secure siguen al TLS, nunca lo preceden.

PROBLEMA QUE RESUELVE: SESSION_COOKIE_SECURE y CSRF_COOKIE_SECURE solo son seguros de
prender DESPUES de que el origen sirva TLS. Al reves, el panel queda INALCANZABLE — el
browser no devuelve una cookie Secure sobre http, asi que nadie puede iniciar sesion — y el
sintoma es un login que "no funciona", sin ningun error que lo explique.

El ticket (DOMAINSERV-204) lo dice literal: "Orden: primero TLS, despues los flags. Al reves
el panel queda inalcanzable". Derivar los flags de ADMIN_DOMAIN convierte ese orden en una
propiedad del codigo en vez de una instruccion que alguien tiene que recordar al editar el
.env — y a un .env ya instalado, install.sh no le agrega claves nuevas.

Ejercita la funcion REAL (config.hardening, que no importa Django) en vez de una copia de
su logica: una copia es lo que diverge y deja el guard en verde midiendo lo que ya no corre.
"""
from __future__ import annotations

import unittest
from pathlib import Path

from config.hardening import flag_de_cookie_segura, hay_tls_en_el_origen

_BASE_PY = Path(__file__).resolve().parent.parent / "settings" / "base.py"


class HayTlsEnElOrigenTests(unittest.TestCase):
    def test_sin_dominio_no_hay_tls(self):
        self.assertFalse(hay_tls_en_el_origen(""))
        self.assertFalse(hay_tls_en_el_origen("   "))
        self.assertFalse(hay_tls_en_el_origen(None))

    def test_admin_localhost_no_cuenta_como_tls(self):
        self.assertFalse(
            hay_tls_en_el_origen("admin.localhost"),
            "admin.localhost es el default del compose para que Caddy levante sin DNS con su "
            "CA interna: no es TLS publico y no habilita las cookies Secure",
        )

    def test_dominio_real_es_tls(self):
        self.assertTrue(hay_tls_en_el_origen("panel.ejemplo.cl"))
        self.assertTrue(hay_tls_en_el_origen("  panel.ejemplo.cl  "))


class FlagDeCookieSeguraTests(unittest.TestCase):
    def test_sin_dominio_y_sin_valor_explicito_queda_apagado(self):
        self.assertFalse(
            flag_de_cookie_segura("", ""),
            "sin ADMIN_DOMAIN el panel se sirve por el bloque :80 en claro: una cookie "
            "Secure no volveria nunca y el login quedaria roto",
        )

    def test_con_dominio_se_prende_solo(self):
        self.assertTrue(
            flag_de_cookie_segura("", "panel.ejemplo.cl"),
            "con dominio Caddy emite el cert automatico: los flags tienen que prenderse sin "
            "que nadie edite el .env, o el hardening queda a medias para siempre",
        )

    def test_el_valor_explicito_gana_sobre_la_derivacion(self):
        self.assertTrue(
            flag_de_cookie_segura("1", ""),
            "un TLS terminado mas arriba no lo puede detectar este proceso: el override "
            "explicito tiene que seguir disponible",
        )
        self.assertFalse(
            flag_de_cookie_segura("0", "panel.ejemplo.cl"),
            "apagarlo explicitamente tiene que funcionar para poder diagnosticar",
        )

    def test_un_valor_que_no_es_1_no_prende_el_flag(self):
        for crudo in ("true", "yes", "si", "2", "on"):
            self.assertFalse(
                flag_de_cookie_segura(crudo, ""),
                f"{crudo!r} no es 1: aceptarlo haria que el flag dependa de como se escriba",
            )


class BasePyUsaLaDerivacionTests(unittest.TestCase):
    def test_base_py_no_defaultea_los_flags_a_cero_fijo(self):
        """El default fijo en "0" era lo que hacia que el flag nunca se prendiera.

        Con os.environ.get(FLAG, "0") el panel podia tener TLS y las cookies seguian viajando
        sin Secure, porque a un .env ya instalado install.sh no le agrega claves: nadie iba a
        poner el 1 a mano.
        """
        fuente = _BASE_PY.read_text(encoding="utf-8")
        for flag in ("SESSION_COOKIE_SECURE", "CSRF_COOKIE_SECURE"):
            self.assertNotIn(
                f'os.environ.get("{flag}", "0")', fuente,
                f"{flag} volvio a tener default fijo en 0: con TLS puesto seguiria mandando "
                "la cookie sin Secure y nada lo avisaria",
            )
            self.assertIn(
                f'os.environ.get("{flag}", "")', fuente,
                f"{flag} tiene que seguir leyendose del entorno con default vacio, que es lo "
                "que dispara la derivacion (y lo que el guard del compose detecta)",
            )
        self.assertEqual(
            2, fuente.count("flag_de_cookie_segura("),
            "los DOS flags tienen que pasar por la derivacion: uno solo dejaria al otro "
            "desalineado, y una sesion Secure con un CSRF que no lo es rompe el login igual",
        )
