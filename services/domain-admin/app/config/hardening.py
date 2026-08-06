"""Derivacion de los flags de cookie segura a partir del dominio del panel.

Vive fuera de config.settings y sin importar Django para que el guard que lo cubre pueda
ejercitar la funcion REAL en vez de una copia de su logica: una copia es exactamente lo que
diverge y deja el test en verde midiendo algo que ya no corre (DOMAINSERV-204).

No va en config/settings/ porque el __init__ de ese package hace `from .base import *`, asi
que cualquier import de un modulo hermano arrastra el settings completo — incluida la
validacion de DJANGO_SECRET_KEY, que en un test no tiene por que estar definida.
"""
from __future__ import annotations


def hay_tls_en_el_origen(admin_domain: str) -> bool:
    """True cuando el panel se sirve sobre TLS publico.

    `admin.localhost` es el default del compose para que Caddy levante sin DNS usando su CA
    interna. NO cuenta como TLS publico: mientras no haya dominio real, el panel se sigue
    sirviendo por el bloque `:80` del Caddyfile y la cookie viaja en claro.
    """
    dominio = (admin_domain or "").strip()
    return bool(dominio) and not dominio.endswith(".localhost")


def flag_de_cookie_segura(valor_crudo: str, admin_domain: str) -> bool:
    """Deriva el flag de admin_domain salvo que el entorno lo fije explicitamente.

    El orden que exige el ticket es TLS primero y flags despues; al reves el panel queda
    INALCANZABLE, porque el browser no devuelve una cookie Secure sobre http y entonces nadie
    puede iniciar sesion. Derivarlo hace que ese orden se cumpla por construccion en vez de
    depender de que alguien lo recuerde al editar el .env — y ese .env, en un ambiente ya
    instalado, install.sh no le agrega claves nuevas.

    El override explicito sigue disponible para el caso de un TLS terminado mas arriba, que
    este proceso no tiene forma de detectar.
    """
    crudo = (valor_crudo or "").strip()
    if crudo == "":
        return hay_tls_en_el_origen(admin_domain)
    return crudo == "1"
