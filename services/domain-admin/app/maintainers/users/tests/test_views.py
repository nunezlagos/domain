"""Tests de las views (HTTP) del mantenedor de usuarios.

Usan el test client real contra URLs reales (namespace 'users', intacto tras la
migración a maintainers.users). Verifican status codes, efectos en DB y forma
de la respuesta (HTML vs JSON vs partial). El helper authenticate() viene de
core.tests.base.MaintainerTestCase.
"""
from __future__ import annotations

import json

from django.test import TestCase
from django.urls import reverse

from core.tests.base import MaintainerTestCase

from maintainers.users.models import User

from .factories import make_role, make_user


class AuthGuardTests(TestCase):
    """Sin sesión autenticada → redirect a /login/ (no toca DB)."""

    def test_list_redirige_sin_auth(self):
        r = self.client.get(reverse("users:list"))
        self.assertEqual(r.status_code, 302)
        self.assertIn("/login/", r["Location"])

    def test_signal_redirige_sin_auth(self):
        r = self.client.get(reverse("users:signal"))
        self.assertEqual(r.status_code, 302)
        self.assertIn("/login/", r["Location"])


class ListViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()
        make_user("vista@example.com", name="Vista User")

    def test_list_ok_muestra_usuario(self):
        r = self.client.get(reverse("users:list"))
        self.assertEqual(r.status_code, 200)
        self.assertContains(r, "vista@example.com")

    def test_fragment_table_devuelve_solo_tabla(self):
        r = self.client.get(reverse("users:list"), {"fragment": "table"})
        self.assertEqual(r.status_code, 200)
        body = r.content.decode()
        self.assertIn("<table", body)
        self.assertNotIn("<html", body)

    def test_search_filtra_server_side(self):
        make_user("otro@example.com", name="Otro")
        r = self.client.get(reverse("users:list"), {"q": "vista@", "fragment": "table"})
        self.assertContains(r, "vista@example.com")
        self.assertNotContains(r, "otro@example.com")


class SignalEndpointTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def test_signal_devuelve_json(self):
        make_user("sig@example.com")
        r = self.client.get(reverse("users:signal"))
        self.assertEqual(r.status_code, 200)
        self.assertEqual(r["Content-Type"], "application/json")
        data = json.loads(r.content)
        self.assertIn("count", data)
        self.assertIn("version", data)
        self.assertEqual(data["count"], 1)


class CreateViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()
        make_role("viewer")

    def test_post_crea_usuario(self):
        r = self.client.post(reverse("users:create"), {
            "email": "creado@example.com",
            "first_name": "Creado",
            "paternal_surname": "Pérez",
            "maternal_surname": "Soto",
            "role": "viewer",
            "status": "active",
            "password": "supersecret",
            "password_confirm": "supersecret",
        })
        self.assertEqual(r.status_code, 302)
        u = User.objects.get(email="creado@example.com")
        self.assertEqual(u.name, "Creado Pérez Soto")

    def test_post_password_corta_no_crea(self):
        r = self.client.post(reverse("users:create"), {
            "email": "corta@example.com",
            "first_name": "Corta",
            "role": "viewer",
            "status": "active",
            "password": "abc",
            "password_confirm": "abc",
        })
        # Form inválido → re-render (200), sin crear.
        self.assertEqual(r.status_code, 200)
        self.assertFalse(User.objects.filter(email="corta@example.com").exists())


class ToggleViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def test_post_toggle_invierte_status(self):
        u = make_user("toggle@example.com", status="active")
        r = self.client.post(reverse("users:toggle", args=[u.pk]),
                             HTTP_X_REQUESTED_WITH="fetch")
        self.assertEqual(r.status_code, 302)
        u.refresh_from_db()
        self.assertEqual(u.status, "suspended")


class DeleteViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def test_post_soft_delete(self):
        u = make_user("del@example.com", status="active")
        r = self.client.post(reverse("users:delete", args=[u.pk]))
        self.assertEqual(r.status_code, 302)
        u.refresh_from_db()
        self.assertEqual(u.status, "revoked")
        self.assertIsNotNone(u.deleted_at)


class DetailViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def test_detail_partial_muestra_usuario(self):
        u = make_user("detalle@example.com", name="Detalle")
        r = self.client.get(reverse("users:detail", args=[u.pk]), {"partial": "1"})
        self.assertEqual(r.status_code, 200)
        body = r.content.decode()
        self.assertIn("detalle@example.com", body)
        self.assertNotIn("<html", body)
