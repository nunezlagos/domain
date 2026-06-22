"""Tests de las views (HTTP) del mantenedor de API Keys.

Usan el test client real contra URLs reales (namespace 'apikeys', intacto tras
la migracion a maintainers.apikeys). El helper authenticate() viene de
core.tests.base.MaintainerTestCase.
"""
from __future__ import annotations

import json

from django.test import TestCase
from django.urls import reverse

from core.tests.base import MaintainerTestCase

from maintainers.apikeys.models import ApiKey

from .factories import make_api_key
from maintainers.users.tests.factories import make_user


class AuthGuardTests(TestCase):
    """Sin sesion autenticada → redirect a /login/ (no toca DB)."""

    def test_list_redirige_sin_auth(self):
        r = self.client.get(reverse("apikeys:list"))
        self.assertEqual(r.status_code, 302)
        self.assertIn("/login/", r["Location"])

    def test_signal_redirige_sin_auth(self):
        r = self.client.get(reverse("apikeys:signal"))
        self.assertEqual(r.status_code, 302)
        self.assertIn("/login/", r["Location"])


class ListViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()
        make_api_key("Vista Key", key_prefix="sk_vista0001")

    def test_list_ok_muestra_key(self):
        r = self.client.get(reverse("apikeys:list"))
        self.assertEqual(r.status_code, 200)
        self.assertContains(r, "Vista Key")

    def test_fragment_table_devuelve_solo_tabla(self):
        r = self.client.get(reverse("apikeys:list"), {"fragment": "table"})
        self.assertEqual(r.status_code, 200)
        body = r.content.decode()
        self.assertIn("<table", body)
        self.assertNotIn("<html", body)

    def test_search_filtra_server_side(self):
        make_api_key("Otra Key", key_prefix="sk_otra00001")
        r = self.client.get(reverse("apikeys:list"), {"q": "Vista", "fragment": "table"})
        self.assertContains(r, "Vista Key")
        self.assertNotContains(r, "Otra Key")


class SignalEndpointTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def test_signal_devuelve_json(self):
        make_api_key("Sig Key")
        r = self.client.get(reverse("apikeys:signal"))
        self.assertEqual(r.status_code, 200)
        self.assertEqual(r["Content-Type"], "application/json")
        data = json.loads(r.content)
        self.assertIn("count", data)
        self.assertIn("version", data)
        self.assertEqual(data["count"], 1)


class DetailViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def test_partial_devuelve_modal(self):
        ak = make_api_key("Detalle Key")
        r = self.client.get(reverse("apikeys:detail", args=[ak.pk]), {"partial": "1"})
        self.assertEqual(r.status_code, 200)
        body = r.content.decode()
        self.assertIn("modal-header", body)
        self.assertNotIn("<html", body)

    def test_detail_inexistente_redirige(self):
        import uuid
        r = self.client.get(reverse("apikeys:detail", args=[uuid.uuid4()]))
        self.assertEqual(r.status_code, 302)
        self.assertIn(reverse("apikeys:list"), r["Location"])


class CreateViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()
        self.owner = make_user("creador@example.com")

    def test_post_crea_key(self):
        r = self.client.post(reverse("apikeys:create"), {
            "name": "Nueva CI",
            "user": str(self.owner.pk),
            "status": "active",
        })
        self.assertEqual(r.status_code, 302)
        self.assertTrue(ApiKey.objects.filter(name="Nueva CI").exists())

    def test_post_nombre_vacio_no_crea(self):
        r = self.client.post(reverse("apikeys:create"), {
            "name": "",
            "user": str(self.owner.pk),
            "status": "active",
        })
        self.assertEqual(r.status_code, 200)  # re-render con errores
        self.assertFalse(ApiKey.objects.filter(user=self.owner).exists())

    def test_post_ajax_devuelve_redirect(self):
        r = self.client.post(
            reverse("apikeys:create"),
            {"name": "Ajax Key", "user": str(self.owner.pk), "status": "active"},
            HTTP_X_REQUESTED_WITH="fetch",
        )
        self.assertEqual(r.status_code, 302)
        self.assertIn(reverse("apikeys:list"), r["Location"])


class EditViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def test_post_edita_nombre(self):
        ak = make_api_key("Original")
        r = self.client.post(reverse("apikeys:edit", args=[ak.pk]), {
            "name": "Editada",
            "status": "active",
        })
        self.assertEqual(r.status_code, 302)
        ak.refresh_from_db()
        self.assertEqual(ak.name, "Editada")


class ToggleViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def test_post_toggle_revoca(self):
        ak = make_api_key("Toggle", status="active")
        r = self.client.post(reverse("apikeys:toggle", args=[ak.pk]),
                             HTTP_X_REQUESTED_WITH="fetch")
        self.assertEqual(r.status_code, 302)
        ak.refresh_from_db()
        self.assertEqual(ak.status, "revoked")


class DeleteViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def test_post_soft_delete(self):
        ak = make_api_key("Borrable", status="active")
        r = self.client.post(reverse("apikeys:delete", args=[ak.pk]))
        self.assertEqual(r.status_code, 302)
        ak.refresh_from_db()
        self.assertEqual(ak.status, "revoked")
        self.assertIsNotNone(ak.revoked_at)
