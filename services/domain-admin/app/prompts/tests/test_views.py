"""Tests de las views (HTTP) del mantenedor de Prompts.

Usan el test client real contra URLs reales. Verifican status codes,
efectos en DB y forma de la respuesta (HTML vs JSON vs partial).
"""
from __future__ import annotations

import json
import uuid

from django.test import TestCase
from django.urls import reverse

from prompts.models import Prompt

from .factories import make_prompt


class AuthGuardTests(TestCase):
    """Sin sesión autenticada → redirect a /login/."""

    def test_list_redirige_sin_auth(self):
        r = self.client.get(reverse("prompts:list"))
        self.assertEqual(r.status_code, 302)
        self.assertIn("/login/", r["Location"])

    def test_signal_redirige_sin_auth(self):
        r = self.client.get(reverse("prompts:signal"))
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
        make_prompt("saludo-vista", body="Hola.")

    def test_list_ok_muestra_prompt(self):
        r = self.client.get(reverse("prompts:list"))
        self.assertEqual(r.status_code, 200)
        self.assertContains(r, "saludo-vista")

    def test_fragment_table_devuelve_solo_tabla(self):
        r = self.client.get(reverse("prompts:list"), {"fragment": "table"})
        self.assertEqual(r.status_code, 200)
        body = r.content.decode()
        self.assertIn("<table", body)
        self.assertNotIn("<html", body)

    def test_search_filtra_server_side(self):
        make_prompt("otro-prompt", body="x")
        r = self.client.get(reverse("prompts:list"),
                             {"q": "saludo-vista", "fragment": "table"})
        self.assertContains(r, "saludo-vista")
        self.assertNotContains(r, "otro-prompt")


class SignalEndpointTests(AuthenticatedMixin, TestCase):
    def setUp(self):
        self.authenticate()

    def test_signal_devuelve_json(self):
        make_prompt("sig")
        r = self.client.get(reverse("prompts:signal"))
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
        p = make_prompt("detalle")
        r = self.client.get(reverse("prompts:detail", args=[p.pk]), {"partial": "1"})
        self.assertEqual(r.status_code, 200)
        body = r.content.decode()
        self.assertIn("modal-header", body)
        self.assertNotIn("<html", body)

    def test_detail_inexistente_redirige(self):
        r = self.client.get(reverse("prompts:detail", args=[uuid.uuid4()]))
        self.assertEqual(r.status_code, 302)


class CreateViewTests(AuthenticatedMixin, TestCase):
    def setUp(self):
        self.authenticate()

    def _data(self, **over):
        base = {
            "project_id": "",
            "slug": "creado",
            "version": "1",
            "body": "Cuerpo del prompt.",
            "description": "una desc",
            "tags": "soporte, ventas",
            "is_active": "on",
        }
        base.update(over)
        return base

    def test_post_crea_prompt(self):
        r = self.client.post(reverse("prompts:create"), self._data())
        self.assertEqual(r.status_code, 302)
        self.assertTrue(Prompt.objects.filter(slug="creado").exists())
        p = Prompt.objects.get(slug="creado")
        self.assertEqual(p.tags, ["soporte", "ventas"])

    def test_post_cuadrupla_duplicada_no_crea(self):
        make_prompt("dup", version=1)
        r = self.client.post(reverse("prompts:create"),
                             self._data(slug="dup", version="1"))
        # Form inválido (clean) → re-render 200, sin crear nuevo.
        self.assertEqual(r.status_code, 200)
        self.assertEqual(Prompt.objects.filter(slug="dup").count(), 1)

    def test_post_ajax_devuelve_redirect(self):
        r = self.client.post(reverse("prompts:create"), self._data(slug="ajax"),
                             HTTP_X_REQUESTED_WITH="fetch")
        self.assertEqual(r.status_code, 302)
        self.assertTrue(Prompt.objects.filter(slug="ajax").exists())


class EditViewTests(AuthenticatedMixin, TestCase):
    def setUp(self):
        self.authenticate()

    def test_post_edita_prompt(self):
        p = make_prompt("orig", body="antes")
        r = self.client.post(reverse("prompts:edit", args=[p.pk]), {
            "project_id": "",
            "slug": "orig",
            "version": "1",
            "body": "despues",
            "description": "",
            "tags": "",
            # is_active omitido → checkbox desmarcado → False
        })
        self.assertEqual(r.status_code, 302)
        p.refresh_from_db()
        self.assertEqual(p.body, "despues")
        self.assertFalse(p.is_active)


class ToggleViewTests(AuthenticatedMixin, TestCase):
    def setUp(self):
        self.authenticate()

    def test_post_toggle_invierte_is_active(self):
        p = make_prompt("toggle", is_active=True)
        r = self.client.post(reverse("prompts:toggle", args=[p.pk]),
                             HTTP_X_REQUESTED_WITH="fetch")
        self.assertEqual(r.status_code, 302)
        p.refresh_from_db()
        self.assertFalse(p.is_active)


class DeleteViewTests(AuthenticatedMixin, TestCase):
    def setUp(self):
        self.authenticate()

    def test_post_soft_delete(self):
        p = make_prompt("del", is_active=True)
        r = self.client.post(reverse("prompts:delete", args=[p.pk]))
        self.assertEqual(r.status_code, 302)
        p.refresh_from_db()
        self.assertFalse(p.is_active)
        self.assertIsNotNone(p.deleted_at)
