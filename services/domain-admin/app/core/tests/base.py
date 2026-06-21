"""TestCase base para mantenedores.

`MaintainerTestCase` agrega el helper `authenticate()` que setea la sesión como
autenticada (igual que el AuthenticatedMixin que está copiado en los test_views
de cada app). Las views de mantenedor exigen sesión autenticada
(core.auth.require_auth), así que casi todos los tests de view lo necesitan.
"""
from __future__ import annotations

from django.test import TestCase


class MaintainerTestCase(TestCase):
    """TestCase con helper de autenticación de sesión."""

    def authenticate(self) -> None:
        """Marca la sesión del test client como autenticada."""
        session = self.client.session
        session["authenticated"] = True
        session.save()
