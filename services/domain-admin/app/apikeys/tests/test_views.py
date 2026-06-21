"""Tests de las views (HTTP) del mantenedor de API Keys.

Usan el test client real contra URLs reales. Verifican status codes, efectos
en DB y forma de la respuesta (HTML vs JSON vs partial).
"""
from __future__ import annotations

import json

from django.test import TestCase
from django.urls import reverse

from apikeys.models import ApiKey

from .factories import make_api_key, make_user


class AuthGuardTests(TestCase):
    """Sin sesión autenticada → redirect a /login/."""

    def test_list_redirige_sin_auth(self):
        r = self.client.get(reverse("apikeys:list"))
        self.assertEqual(r.status_code, 302)
        self.assertIn("/login/", r["Location"])

    def test_signal_redirige_sin_auth(self):
        r = self.client.get(reverse("apikeys:signal"))
        self.assertEqual(r.status_code, 302)
        self.assertIn("/login/", r["Location"])


class AuthenticatedMixin:
    def authenticate(self):
        session = self.client.session
        session["authenticated"] = True
        session.save()


class ListViewTests(AuthenticatedMixin, TestCase):
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


class SignalEndpointTests(AuthenticatedMixin, TestCase):
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


class DetailViewTests(AuthenticatedMixin, TestCase):
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


class CreateViewTests(AuthenticatedMixin, TestCase):
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


class EditViewTests(AuthenticatedMixin, TestCase):
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


class ToggleViewTests(AuthenticatedMixin, TestCase):
    def setUp(self):
        self.authenticate()

    def test_post_toggle_revoca(self):
        ak = make_api_key("Toggle", status="active")
        r = self.client.post(reverse("apikeys:toggle", args=[ak.pk]),
                             HTTP_X_REQUESTED_WITH="fetch")
        self.assertEqual(r.status_code, 302)
        ak.refresh_from_db()
        self.assertEqual(ak.status, "revoked")


class DeleteViewTests(AuthenticatedMixin, TestCase):
    def setUp(self):
        self.authenticate()

    def test_post_soft_delete(self):
        ak = make_api_key("Borrable", status="active")
        r = self.client.post(reverse("apikeys:delete", args=[ak.pk]))
        self.assertEqual(r.status_code, 302)
        ak.refresh_from_db()
        self.assertEqual(ak.status, "revoked")
        self.assertIsNotNone(ak.revoked_at)
