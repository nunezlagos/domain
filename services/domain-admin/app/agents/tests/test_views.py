"""Tests de las views (HTTP) del mantenedor de Agentes.

Usan el test client real contra URLs reales. Verifican status codes,
efectos en DB y forma de la respuesta (HTML vs JSON vs partial).
"""
from __future__ import annotations

import json
import uuid

from django.test import TestCase
from django.urls import reverse

from agents.models import Agent

from .factories import DEFAULT_ORG, make_agent, make_agent_version


class AuthGuardTests(TestCase):
    """Sin sesión autenticada → redirect a /login/."""

    def test_list_redirige_sin_auth(self):
        r = self.client.get(reverse("agents:list"))
        self.assertEqual(r.status_code, 302)
        self.assertIn("/login/", r["Location"])

    def test_signal_redirige_sin_auth(self):
        r = self.client.get(reverse("agents:signal"))
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
        make_agent("Vista Bot", slug="vista", provider="anthropic")

    def test_list_ok_muestra_agente(self):
        r = self.client.get(reverse("agents:list"))
        self.assertEqual(r.status_code, 200)
        self.assertContains(r, "Vista Bot")

    def test_fragment_table_devuelve_solo_tabla(self):
        r = self.client.get(reverse("agents:list"), {"fragment": "table"})
        self.assertEqual(r.status_code, 200)
        body = r.content.decode()
        self.assertIn("<table", body)
        self.assertNotIn("<html", body)

    def test_search_filtra_server_side(self):
        make_agent("Otro Bot", slug="otro")
        r = self.client.get(reverse("agents:list"), {"q": "Vista", "fragment": "table"})
        self.assertContains(r, "Vista Bot")
        self.assertNotContains(r, "Otro Bot")


class SignalEndpointTests(AuthenticatedMixin, TestCase):
    def setUp(self):
        self.authenticate()

    def test_signal_devuelve_json(self):
        make_agent("Sig", slug="sig")
        r = self.client.get(reverse("agents:signal"))
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
        a = make_agent("Detalle", slug="detalle")
        r = self.client.get(reverse("agents:detail", args=[a.pk]), {"partial": "1"})
        self.assertEqual(r.status_code, 200)
        body = r.content.decode()
        self.assertIn("modal-header", body)
        self.assertNotIn("<html", body)

    def test_detail_partial_lista_versiones(self):
        a = make_agent("ConVersiones", slug="conv")
        make_agent_version(a, 1)
        make_agent_version(a, 2)
        r = self.client.get(reverse("agents:detail", args=[a.pk]), {"partial": "1"})
        body = r.content.decode()
        self.assertIn("Historial de versiones", body)
        self.assertIn("v2", body)

    def test_detail_inexistente_redirige(self):
        r = self.client.get(reverse("agents:detail", args=[uuid.uuid4()]))
        self.assertEqual(r.status_code, 302)


class CreateViewTests(AuthenticatedMixin, TestCase):
    def setUp(self):
        self.authenticate()

    def _data(self, **over):
        base = {
            "organization_id": str(DEFAULT_ORG),
            "name": "Creado Bot",
            "slug": "creado",
            "provider": "anthropic",
            "model": "claude-haiku-4-5",
            "description": "",
            "system_prompt": "",
            "skills_slugs": "search, summarize",
            "max_iterations": "20",
            "token_budget": "",
            "temperature": "",
        }
        base.update(over)
        return base

    def test_post_crea_agente(self):
        r = self.client.post(reverse("agents:create"), self._data())
        self.assertEqual(r.status_code, 302)
        self.assertTrue(Agent.objects.filter(slug="creado").exists())
        a = Agent.objects.get(slug="creado")
        self.assertEqual(a.skills_slugs, ["search", "summarize"])

    def test_post_slug_duplicado_no_crea(self):
        make_agent("Existente", slug="dup")
        r = self.client.post(reverse("agents:create"), self._data(slug="dup"))
        # Form inválido (clean_slug) → re-render 200, sin crear nuevo.
        self.assertEqual(r.status_code, 200)
        self.assertEqual(Agent.objects.filter(slug="dup").count(), 1)

    def test_post_campos_requeridos_faltantes_no_crea(self):
        r = self.client.post(reverse("agents:create"),
                             self._data(provider="", model=""))
        self.assertEqual(r.status_code, 200)
        self.assertFalse(Agent.objects.filter(slug="creado").exists())

    def test_post_ajax_devuelve_redirect(self):
        r = self.client.post(reverse("agents:create"), self._data(slug="ajax"),
                             HTTP_X_REQUESTED_WITH="fetch")
        self.assertEqual(r.status_code, 302)
        self.assertTrue(Agent.objects.filter(slug="ajax").exists())


class EditViewTests(AuthenticatedMixin, TestCase):
    def setUp(self):
        self.authenticate()

    def test_post_edita_agente(self):
        a = make_agent("Original", slug="orig", provider="anthropic")
        r = self.client.post(reverse("agents:edit", args=[a.pk]), {
            "organization_id": str(a.organization_id),
            "name": "Editado",
            "slug": "orig",
            "provider": "openai",
            "model": "gpt-4o",
            "description": "",
            "system_prompt": "",
            "skills_slugs": "",
            "max_iterations": "30",
            "token_budget": "",
            "temperature": "",
        })
        self.assertEqual(r.status_code, 302)
        a.refresh_from_db()
        self.assertEqual(a.name, "Editado")
        self.assertEqual(a.provider, "openai")
        self.assertEqual(a.max_iterations, 30)


class DeleteViewTests(AuthenticatedMixin, TestCase):
    def setUp(self):
        self.authenticate()

    def test_post_soft_delete(self):
        a = make_agent("Del", slug="del")
        r = self.client.post(reverse("agents:delete", args=[a.pk]))
        self.assertEqual(r.status_code, 302)
        a.refresh_from_db()
        self.assertIsNotNone(a.deleted_at)
        self.assertTrue(Agent.objects.filter(pk=a.pk).exists())
