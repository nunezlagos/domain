"""Tests de las views (HTTP) del mantenedor de Clientes.

Usan el test client real contra URLs reales. Verifican status codes,
efectos en DB y forma de la respuesta (HTML vs JSON vs partial).
"""
from __future__ import annotations

import json
import uuid

from django.test import TestCase
from django.urls import reverse

from clients.models import Client

from .factories import DEFAULT_ORG, make_client


class AuthGuardTests(TestCase):
    """Sin sesión autenticada → redirect a /login/."""

    def test_list_redirige_sin_auth(self):
        r = self.client.get(reverse("clients:list"))
        self.assertEqual(r.status_code, 302)
        self.assertIn("/login/", r["Location"])

    def test_signal_redirige_sin_auth(self):
        r = self.client.get(reverse("clients:signal"))
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
        make_client("Vista Corp", slug="vista", contact_email="ops@vista.com")

    def test_list_ok_muestra_cliente(self):
        r = self.client.get(reverse("clients:list"))
        self.assertEqual(r.status_code, 200)
        self.assertContains(r, "Vista Corp")

    def test_fragment_table_devuelve_solo_tabla(self):
        r = self.client.get(reverse("clients:list"), {"fragment": "table"})
        self.assertEqual(r.status_code, 200)
        body = r.content.decode()
        self.assertIn("<table", body)
        self.assertNotIn("<html", body)

    def test_search_filtra_server_side(self):
        make_client("Otro Cliente", slug="otro")
        r = self.client.get(reverse("clients:list"), {"q": "Vista", "fragment": "table"})
        self.assertContains(r, "Vista Corp")
        self.assertNotContains(r, "Otro Cliente")


class SignalEndpointTests(AuthenticatedMixin, TestCase):
    def setUp(self):
        self.authenticate()

    def test_signal_devuelve_json(self):
        make_client("Sig", slug="sig")
        r = self.client.get(reverse("clients:signal"))
        self.assertEqual(r.status_code, 200)
        self.assertEqual(r["Content-Type"], "application/json")
        data = json.loads(r.content)
        self.assertIn("count", data)
        self.assertIn("version", data)
        self.assertEqual(data["count"], 1)


class DetailViewTests(AuthenticatedMixin, TestCase):
    def setUp(self):
        self.authenticate()

    def test_detail_partial_devuelve_modal(self):
        c = make_client("Detalle", slug="detalle")
        r = self.client.get(reverse("clients:detail", args=[c.pk]), {"partial": "1"})
        self.assertEqual(r.status_code, 200)
        body = r.content.decode()
        self.assertIn("modal-header", body)
        self.assertNotIn("<html", body)

    def test_detail_inexistente_redirige(self):
        r = self.client.get(reverse("clients:detail", args=[uuid.uuid4()]))
        self.assertEqual(r.status_code, 302)


class CreateViewTests(AuthenticatedMixin, TestCase):
    def setUp(self):
        self.authenticate()

    def _data(self, **over):
        base = {
            "organization_id": str(DEFAULT_ORG),
            "name": "Creado SpA",
            "slug": "creado",
            "tax_id": "",
            "contact_email": "ops@creado.com",
            "contact_phone": "",
            "address": "",
            "status": "active",
        }
        base.update(over)
        return base

    def test_post_crea_cliente(self):
        r = self.client.post(reverse("clients:create"), self._data())
        self.assertEqual(r.status_code, 302)
        self.assertTrue(Client.objects.filter(slug="creado").exists())

    def test_post_slug_duplicado_no_crea(self):
        make_client("Existente", slug="dup")
        r = self.client.post(reverse("clients:create"), self._data(slug="dup"))
        # Form inválido (clean_slug) → re-render 200, sin crear nuevo.
        self.assertEqual(r.status_code, 200)
        self.assertEqual(Client.objects.filter(slug="dup").count(), 1)

    def test_post_ajax_devuelve_redirect(self):
        r = self.client.post(reverse("clients:create"), self._data(slug="ajax"),
                             HTTP_X_REQUESTED_WITH="fetch")
        self.assertEqual(r.status_code, 302)
        self.assertTrue(Client.objects.filter(slug="ajax").exists())


class EditViewTests(AuthenticatedMixin, TestCase):
    def setUp(self):
        self.authenticate()

    def test_post_edita_cliente(self):
        c = make_client("Original", slug="orig")
        r = self.client.post(reverse("clients:edit", args=[c.pk]), {
            "organization_id": str(c.organization_id),
            "name": "Editado",
            "slug": "orig",
            "tax_id": "",
            "contact_email": "",
            "contact_phone": "",
            "address": "",
            "status": "inactive",
        })
        self.assertEqual(r.status_code, 302)
        c.refresh_from_db()
        self.assertEqual(c.name, "Editado")
        self.assertEqual(c.status, "inactive")


class ToggleViewTests(AuthenticatedMixin, TestCase):
    def setUp(self):
        self.authenticate()

    def test_post_toggle_invierte_status(self):
        c = make_client("Toggle", slug="toggle", status="active")
        r = self.client.post(reverse("clients:toggle", args=[c.pk]),
                             HTTP_X_REQUESTED_WITH="fetch")
        self.assertEqual(r.status_code, 302)
        c.refresh_from_db()
        self.assertEqual(c.status, "inactive")


class DeleteViewTests(AuthenticatedMixin, TestCase):
    def setUp(self):
        self.authenticate()

    def test_post_soft_delete(self):
        c = make_client("Del", slug="del", status="active")
        r = self.client.post(reverse("clients:delete", args=[c.pk]))
        self.assertEqual(r.status_code, 302)
        c.refresh_from_db()
        self.assertEqual(c.status, "archived")
        self.assertIsNotNone(c.deleted_at)
