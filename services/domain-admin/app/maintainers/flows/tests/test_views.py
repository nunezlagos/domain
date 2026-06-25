"""Tests de las views (HTTP) del mantenedor de Flows (migrado a core).

Usan el test client real contra URLs reales (namespace 'flows', intacto tras la
migracion a maintainers.flows). Verifican status codes, efectos en DB y forma
de la respuesta (HTML vs JSON vs partial). El helper authenticate() viene de
core.tests.base.MaintainerTestCase.
"""
from __future__ import annotations

import json
import uuid

from django.test import TestCase
from django.urls import reverse

from core.tests.base import MaintainerTestCase

from maintainers.flows.models import Flow

from .factories import make_flow, make_flow_version


class AuthGuardTests(TestCase):
    """Sin sesion autenticada → redirect a /login/ (no toca DB)."""

    def test_list_redirige_sin_auth(self):
        r = self.client.get(reverse("flows:list"))
        self.assertEqual(r.status_code, 302)
        self.assertIn("/login/", r["Location"])

    def test_signal_redirige_sin_auth(self):
        r = self.client.get(reverse("flows:signal"))
        self.assertEqual(r.status_code, 302)
        self.assertIn("/login/", r["Location"])


class ListViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()
        make_flow("Vista Flow", slug="vista", description="visible")

    def test_list_ok_muestra_flow(self):
        r = self.client.get(reverse("flows:list"))
        self.assertEqual(r.status_code, 200)
        self.assertContains(r, "Vista Flow")

    def test_fragment_table_devuelve_solo_tabla(self):
        r = self.client.get(reverse("flows:list"), {"fragment": "table"})
        self.assertEqual(r.status_code, 200)
        body = r.content.decode()
        self.assertIn("<table", body)
        self.assertNotIn("<html", body)

    def test_search_filtra_server_side(self):
        make_flow("Otro Flow", slug="otro")
        r = self.client.get(reverse("flows:list"), {"q": "Vista", "fragment": "table"})
        self.assertContains(r, "Vista Flow")
        self.assertNotContains(r, "Otro Flow")


class SignalEndpointTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def test_signal_devuelve_json(self):
        make_flow("Sig", slug="sig")
        r = self.client.get(reverse("flows:signal"))
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
        f = make_flow("Detalle", slug="detalle")
        r = self.client.get(reverse("flows:detail", args=[f.pk]), {"partial": "1"})
        self.assertEqual(r.status_code, 200)
        body = r.content.decode()
        self.assertIn("modal-header", body)
        self.assertNotIn("<html", body)

    def test_detail_lista_versiones_readonly(self):
        f = make_flow("ConVersiones", slug="conversiones")
        make_flow_version(f, version=1, note="primera")
        make_flow_version(f, version=2, note="segunda")
        r = self.client.get(reverse("flows:detail", args=[f.pk]), {"partial": "1"})
        body = r.content.decode()
        self.assertIn("v2", body)
        self.assertIn("segunda", body)

    def test_detail_inexistente_redirige(self):
        r = self.client.get(reverse("flows:detail", args=[uuid.uuid4()]))
        self.assertEqual(r.status_code, 302)


class CreateViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def _data(self, **over):
        base = {
            "name": "Creado Flow",
            "slug": "creado",
            "description": "",
            "spec": "{}",
            "is_active": "on",
            "seed_version": "",
        }
        base.update(over)
        return base

    def test_post_crea_flow(self):
        r = self.client.post(reverse("flows:create"), self._data())
        self.assertEqual(r.status_code, 302)
        self.assertTrue(Flow.objects.filter(slug="creado").exists())

    def test_post_spec_invalido_no_crea(self):
        r = self.client.post(reverse("flows:create"), self._data(spec="no-es-json"))
        self.assertEqual(r.status_code, 200)
        self.assertFalse(Flow.objects.filter(slug="creado").exists())

    def test_post_slug_duplicado_no_crea(self):
        make_flow("Existente", slug="dup")
        r = self.client.post(reverse("flows:create"), self._data(slug="dup"))

        self.assertEqual(r.status_code, 200)
        self.assertEqual(Flow.objects.filter(slug="dup").count(), 1)

    def test_post_ajax_devuelve_redirect(self):
        r = self.client.post(reverse("flows:create"), self._data(slug="ajax"),
                             HTTP_X_REQUESTED_WITH="fetch")
        self.assertEqual(r.status_code, 302)
        self.assertTrue(Flow.objects.filter(slug="ajax").exists())


class EditViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def test_post_edita_flow(self):
        f = make_flow("Original", slug="orig", is_active=True)
        r = self.client.post(reverse("flows:edit", args=[f.pk]), {
            "name": "Editado",
            "slug": "orig",
            "description": "",
            "spec": "{}",
            "seed_version": "",

        })
        self.assertEqual(r.status_code, 302)
        f.refresh_from_db()
        self.assertEqual(f.name, "Editado")
        self.assertFalse(f.is_active)


class ToggleViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def test_post_toggle_invierte_is_active(self):
        f = make_flow("Toggle", slug="toggle", is_active=True)
        r = self.client.post(reverse("flows:toggle", args=[f.pk]),
                             HTTP_X_REQUESTED_WITH="fetch")
        self.assertEqual(r.status_code, 302)
        f.refresh_from_db()
        self.assertFalse(f.is_active)


class DeleteViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def test_post_soft_delete(self):
        f = make_flow("Del", slug="del", is_active=True)
        r = self.client.post(reverse("flows:delete", args=[f.pk]))
        self.assertEqual(r.status_code, 302)
        f.refresh_from_db()
        self.assertFalse(f.is_active)
        self.assertIsNotNone(f.deleted_at)
