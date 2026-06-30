"""TestCase base para mantenedores.

`MaintainerTestCase` agrega el helper `authenticate()` que setea la sesion como
autenticada (igual que el AuthenticatedMixin que esta copiado en los test_views
de cada app). Las views de mantenedor exigen sesion autenticada
(core.auth.require_auth), asi que casi todos los tests de view lo necesitan.
"""
from __future__ import annotations

from django.test import TestCase


class MaintainerTestCase(TestCase):
    """TestCase con helper de autenticacion de sesion."""

    def authenticate(self) -> None:
        """Marca la sesion del test client como autenticada."""
        session = self.client.session
        session["authenticated"] = True
        session.save()
