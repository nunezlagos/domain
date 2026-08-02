"""Guard: los knobs de hardening del panel viajan del compose al settings.

PROBLEMA QUE RESUELVE: base.py leia SESSION_COOKIE_SECURE del entorno pero
services/domain-admin/docker-compose.yml nunca pasaba esa variable. En prod la
variable no existia en el proceso del container, asi que el flag era
INALCANZABLE: ponerlo en el .env no tenia ningun efecto y el settings mentia
sobre lo que soportaba (DOMAINSERV-204). Es exactamente el modo de falla que
documenta la policy `guards-deben-ejecutarse`: codigo que soporta el flag +
ambiente que no lo define = flag apagado y nadie avisa.

POR QUE EL COMPOSE Y NO EL .env.example: install.sh, sobre un .env que ya
existe, solo re-siembra las claves del array CREDS. Una variable nueva NO
secreta jamas se agrega al .env de produccion, asi que el default del compose
es el UNICO valor que llega al VPS.
"""
from __future__ import annotations

import re
import unittest
from pathlib import Path

_APP = Path(__file__).resolve().parent.parent.parent
_BASE_PY = _APP / "config" / "settings" / "base.py"
_COMPOSE = _APP.parent / "docker-compose.yml"

# los knobs que deciden si la sesion del panel se puede robar o suplantar:
# el settings los tiene que leer del entorno Y el compose los tiene que pasar
_KNOBS_DE_HARDENING = (
    "DJANGO_SECRET_KEY",
    "DJANGO_ALLOWED_HOSTS",
    "DJANGO_CSRF_TRUSTED_ORIGINS",
    "SESSION_COOKIE_SECURE",
    "CSRF_COOKIE_SECURE",
)

# deuda previa a DOMAINSERV-204: el settings las lee, el compose no las pasa, y
# por eso hoy son inalcanzables en prod. Se listan para que el cierre de abajo
# no las tape. Agregar una var de seguridad aca en vez de al compose es
# justamente lo que este guard existe para impedir
_INALCANZABLES_CONOCIDAS = frozenset({
    "DJANGO_ENV",
    "DJANGO_SETTINGS_MODULE",
    "DJANGO_DEBUG",
    "GOOGLE_CLIENT_ID",
    "DEFAULT_ORG_ID",
    "DOMAIN_MCP_HEALTH_URL",
    "DOMAIN_BASE_URL",
    "DOMAIN_API_BASE_URL",
    "DOMAIN_API_TOKEN",
})

_LEE_DEL_ENTORNO = re.compile(r"os\.environ\.get\(\s*[\"']([A-Z][A-Z0-9_]*)[\"']")
_CLAVE_DEL_COMPOSE = re.compile(r"^ {6}([A-Z][A-Z0-9_]*):")
_IPV4 = re.compile(r"\b\d{1,3}(?:\.\d{1,3}){3}\b")


def _vars_que_lee_el_settings() -> set[str]:
    return set(_LEE_DEL_ENTORNO.findall(_BASE_PY.read_text(encoding="utf-8")))


def _vars_que_pasa_el_compose() -> set[str]:
    claves: set[str] = set()
    dentro = False
    for linea in _COMPOSE.read_text(encoding="utf-8").splitlines():
        if linea.strip() == "environment:":
            dentro = True
            continue
        if dentro:
            match = _CLAVE_DEL_COMPOSE.match(linea)
            if match:
                claves.add(match.group(1))
            elif linea.strip() and not linea.startswith(" " * 6):
                dentro = False
    return claves


class HardeningEnvLlegaDelComposeTests(unittest.TestCase):
    def test_cada_knob_de_hardening_viaja_del_compose_al_settings(self):
        leidas = _vars_que_lee_el_settings()
        pasadas = _vars_que_pasa_el_compose()
        for knob in _KNOBS_DE_HARDENING:
            self.assertIn(
                knob, leidas,
                f"{knob} no se lee del entorno en base.py: con el valor "
                "hardcodeado, el compose no lo puede cambiar",
            )
            self.assertIn(
                knob, pasadas,
                f"{knob} no esta en el environment: del compose de "
                "domain-admin. El settings la lee pero en prod la variable no "
                "existe: el knob es INALCANZABLE, no 'apagado'",
            )

    def test_el_settings_no_hardcodea_hosts_del_deployment(self):
        fuente = _BASE_PY.read_text(encoding="utf-8")
        self.assertIsNone(
            _IPV4.search(fuente),
            "base.py trae una IP literal: el host del deployment va en el "
            "environment: del compose, no en el codigo",
        )
        self.assertFalse(
            'ALLOWED_HOSTS = ["*"]' in fuente,
            "base.py vuelve a traer ALLOWED_HOSTS = ['*'], que desactiva la "
            "validacion de Host: en este stack es la unica que hay, porque el "
            "bloque :80 del Caddyfile no matchea por Host",
        )

    def test_ninguna_var_nueva_del_settings_queda_sin_pasar_por_el_compose(self):
        huerfanas = (
            _vars_que_lee_el_settings()
            - _vars_que_pasa_el_compose()
            - _INALCANZABLES_CONOCIDAS
        )
        self.assertEqual(
            set(), huerfanas,
            "base.py lee estas variables y el compose no las pasa: en prod "
            "quedan en su default y el .env no las puede cambiar. Pasalas por "
            "el compose, o sumalas a _INALCANZABLES_CONOCIDAS con su razon",
        )
