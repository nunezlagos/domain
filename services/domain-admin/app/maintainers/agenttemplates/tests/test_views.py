"""Tests de las views (HTTP) del mantenedor de Plantillas de Agentes.

Usan el test client real contra URLs reales (namespace 'agenttemplates').
Verifican status codes, efectos en DB y forma de la respuesta (HTML vs JSON vs
partial). El helper authenticate() viene de core.tests.base.MaintainerTestCase.
"""
from __future__ import annotations

import json
import uuid

from django.test import TestCase
from django.urls import reverse

from core.tests.base import MaintainerTestCase

from maintainers.agenttemplates.models import AgentTemplate

from .factories import make_agent_template


class AuthGuardTests(TestCase):
    """Sin sesion autenticada → redirect a /login/ (no toca DB)."""

    def test_list_redirige_sin_auth(self):
        r = self.client.get(reverse("agenttemplates:list"))
        self.assertEqual(r.status_code, 302)
        self.assertIn("/login/", r["Location"])

    def test_signal_redirige_sin_auth(self):
        r = self.client.get(reverse("agenttemplates:signal"))
        self.assertEqual(r.status_code, 302)
        self.assertIn("/login/", r["Location"])


class ListViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()
        make_agent_template("Vista Plantilla", slug="vista")

    def test_list_ok_muestra_template(self):
        r = self.client.get(reverse("agenttemplates:list"))
        self.assertEqual(r.status_code, 200)
        self.assertContains(r, "Vista Plantilla")

    def test_fragment_table_devuelve_solo_tabla(self):
        r = self.client.get(reverse("agenttemplates:list"), {"fragment": "table"})
        self.assertEqual(r.status_code, 200)
        body = r.content.decode()
        self.assertIn("<table", body)
        self.assertNotIn("<html", body)

    def test_search_filtra_server_side(self):
        make_agent_template("Otra Plantilla", slug="otra")
        r = self.client.get(reverse("agenttemplates:list"),
                            {"q": "Vista", "fragment": "table"})
        self.assertContains(r, "Vista Plantilla")
        self.assertNotContains(r, "Otra Plantilla")


class SignalEndpointTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def test_signal_devuelve_json(self):
        make_agent_template("Sig", slug="sig")
        r = self.client.get(reverse("agenttemplates:signal"))
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
        t = make_agent_template("Detalle", slug="detalle")
        r = self.client.get(reverse("agenttemplates:detail", args=[t.pk]),
                            {"partial": "1"})
        self.assertEqual(r.status_code, 200)
        body = r.content.decode()
        self.assertIn("modal-header", body)
        self.assertNotIn("<html", body)

    def test_detail_inexistente_redirige(self):
        r = self.client.get(reverse("agenttemplates:detail", args=[uuid.uuid4()]))
        self.assertEqual(r.status_code, 302)


class CreateViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def _data(self, **over):
        base = {
            "name": "Creada",
            "slug": "creada",
            "system_prompt": "Sos util.",
            "personality": "amable",
            "capabilities": "research, code",
            "model": "claude-haiku-4-5",
            "temperature": "0.7",
            "max_tokens": "4096",
            "handoff_policy": "allow",
            "role": "phase-worker",
        }
        base.update(over)
        return base

    def test_post_crea_template(self):
        r = self.client.post(reverse("agenttemplates:create"), self._data())
        self.assertEqual(r.status_code, 302)
        self.assertTrue(AgentTemplate.objects.filter(slug="creada").exists())

    def test_post_capabilities_se_parsean(self):
        self.client.post(reverse("agenttemplates:create"), self._data(slug="concaps"))
        t = AgentTemplate.objects.get(slug="concaps")
        self.assertEqual(t.capabilities, ["research", "code"])

    def test_post_slug_duplicado_no_crea(self):
        make_agent_template("Existente", slug="dup")
        r = self.client.post(reverse("agenttemplates:create"), self._data(slug="dup"))

        self.assertEqual(r.status_code, 200)
        self.assertEqual(AgentTemplate.objects.filter(slug="dup").count(), 1)

    def test_post_ajax_devuelve_redirect(self):
        r = self.client.post(reverse("agenttemplates:create"), self._data(slug="ajax"),
                             HTTP_X_REQUESTED_WITH="fetch")
        self.assertEqual(r.status_code, 302)
        self.assertTrue(AgentTemplate.objects.filter(slug="ajax").exists())


class EditViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def test_post_edita_template(self):
        t = make_agent_template("Original", slug="orig", max_tokens=4096)
        r = self.client.post(reverse("agenttemplates:edit", args=[t.pk]), {
            "name": "Editada",
            "slug": "orig",
            "system_prompt": "nuevo",
            "personality": "",
            "capabilities": "",
            "model": "claude-haiku-4-5",
            "temperature": "0.5",
            "max_tokens": "8192",
            "handoff_policy": "forbid",
            "role": "orchestrator",
        })
        self.assertEqual(r.status_code, 302)
        t.refresh_from_db()
        self.assertEqual(t.name, "Editada")
        self.assertEqual(t.max_tokens, 8192)
        self.assertEqual(t.role, "orchestrator")


class DeleteViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def test_post_hard_delete(self):
        t = make_agent_template("Del", slug="del")
        pk = t.pk
        r = self.client.post(reverse("agenttemplates:delete", args=[pk]))
        self.assertEqual(r.status_code, 302)
        self.assertFalse(AgentTemplate.objects.filter(pk=pk).exists())
