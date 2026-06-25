"""Tests de las views (HTTP) del mantenedor de Crons.

Usan el test client real contra URLs reales (namespace 'crons', intacto tras la
migracion a maintainers.crons). Verifican status codes, efectos en DB y forma de
la respuesta (HTML vs JSON vs partial). El helper authenticate() viene de
core.tests.base.MaintainerTestCase.
"""
from __future__ import annotations

import json
import uuid

from django.test import TestCase
from django.urls import reverse

from core.tests.base import MaintainerTestCase

from maintainers.crons.models import Cron

from .factories import DEFAULT_TARGET, make_cron


class AuthGuardTests(TestCase):
    """Sin sesion autenticada → redirect a /login/ (no toca DB)."""

    def test_list_redirige_sin_auth(self):
        r = self.client.get(reverse("crons:list"))
        self.assertEqual(r.status_code, 302)
        self.assertIn("/login/", r["Location"])

    def test_signal_redirige_sin_auth(self):
        r = self.client.get(reverse("crons:signal"))
        self.assertEqual(r.status_code, 302)
        self.assertIn("/login/", r["Location"])


class ListViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()
        make_cron("Daily Report", slug="daily-report")

    def test_list_ok_muestra_cron(self):
        r = self.client.get(reverse("crons:list"))
        self.assertEqual(r.status_code, 200)
        self.assertContains(r, "Daily Report")

    def test_fragment_table_devuelve_solo_tabla(self):
        r = self.client.get(reverse("crons:list"), {"fragment": "table"})
        self.assertEqual(r.status_code, 200)
        body = r.content.decode()
        self.assertIn("<table", body)
        self.assertNotIn("<html", body)

    def test_search_filtra_server_side(self):
        make_cron("Otro Cron", slug="otro")
        r = self.client.get(reverse("crons:list"), {"q": "Daily", "fragment": "table"})
        self.assertContains(r, "Daily Report")
        self.assertNotContains(r, "Otro Cron")


class SignalEndpointTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def test_signal_devuelve_json(self):
        make_cron("Sig", slug="sig")
        r = self.client.get(reverse("crons:signal"))
        self.assertEqual(r.status_code, 200)
        self.assertEqual(r["Content-Type"], "application/json")
        data = json.loads(r.content)
        self.assertIn("count", data)
        self.assertIn("version", data)
        self.assertEqual(data["count"], 1)


class DetailViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def test_detail_partial_devuelve_modal(self):
        c = make_cron("Detalle", slug="detalle")
        r = self.client.get(reverse("crons:detail", args=[c.pk]), {"partial": "1"})
        self.assertEqual(r.status_code, 200)
        body = r.content.decode()
        self.assertIn("modal-header", body)
        self.assertNotIn("<html", body)

    def test_detail_inexistente_redirige(self):
        r = self.client.get(reverse("crons:detail", args=[uuid.uuid4()]))
        self.assertEqual(r.status_code, 302)


class CreateViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def _data(self, **over):
        base = {
            "name": "Creado Cron",
            "slug": "creado",
            "description": "",
            "cron_expression": "0 9 * * *",
            "timezone": "UTC",
            "target_type": "flow",
            "target_id": str(DEFAULT_TARGET),
            "inputs": "{}",
            "enabled": "on",
        }
        base.update(over)
        return base

    def test_post_crea_cron(self):
        r = self.client.post(reverse("crons:create"), self._data())
        self.assertEqual(r.status_code, 302)
        self.assertTrue(Cron.objects.filter(slug="creado").exists())

    def test_post_slug_duplicado_no_crea(self):
        make_cron("Existente", slug="dup")
        r = self.client.post(reverse("crons:create"), self._data(slug="dup"))

        self.assertEqual(r.status_code, 200)
        self.assertEqual(Cron.objects.filter(slug="dup").count(), 1)

    def test_post_inputs_invalido_no_crea(self):
        r = self.client.post(reverse("crons:create"),
                             self._data(slug="badjson", inputs="no-es-json"))
        self.assertEqual(r.status_code, 200)
        self.assertFalse(Cron.objects.filter(slug="badjson").exists())

    def test_post_ajax_devuelve_redirect(self):
        r = self.client.post(reverse("crons:create"), self._data(slug="ajax"),
                             HTTP_X_REQUESTED_WITH="fetch")
        self.assertEqual(r.status_code, 302)
        self.assertTrue(Cron.objects.filter(slug="ajax").exists())


class EditViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def test_post_edita_cron(self):
        c = make_cron("Original", slug="orig", enabled=True)
        r = self.client.post(reverse("crons:edit", args=[c.pk]), {
            "name": "Editado",
            "slug": "orig",
            "description": "",
            "cron_expression": "0 12 * * *",
            "timezone": "UTC",
            "target_type": "agent",
            "target_id": str(DEFAULT_TARGET),
            "inputs": "{}",

        })
        self.assertEqual(r.status_code, 302)
        c.refresh_from_db()
        self.assertEqual(c.name, "Editado")
        self.assertEqual(c.cron_expression, "0 12 * * *")
        self.assertEqual(c.target_type, "agent")
        self.assertFalse(c.enabled)


class ToggleViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def test_post_toggle_invierte_enabled(self):
        c = make_cron("Toggle", slug="toggle", enabled=True)
        r = self.client.post(reverse("crons:toggle", args=[c.pk]),
                             HTTP_X_REQUESTED_WITH="fetch")
        self.assertEqual(r.status_code, 302)
        c.refresh_from_db()
        self.assertFalse(c.enabled)


class DeleteViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def test_post_soft_delete(self):
        c = make_cron("Del", slug="del", enabled=True)
        r = self.client.post(reverse("crons:delete", args=[c.pk]))
        self.assertEqual(r.status_code, 302)
        c.refresh_from_db()
        self.assertFalse(c.enabled)
        self.assertIsNotNone(c.deleted_at)
