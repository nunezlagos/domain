"""Tests de las views (HTTP) del mantenedor de Skills.

Usan el test client real contra URLs reales (namespace 'skills', intacto tras la
migración a maintainers.skills). Verifican status codes, efectos en DB y forma
de la respuesta (HTML vs JSON vs partial). El helper authenticate() viene de
core.tests.base.MaintainerTestCase.
"""
from __future__ import annotations

import json
import uuid

from django.test import TestCase
from django.urls import reverse

from core.tests.base import MaintainerTestCase

from maintainers.skills.models import Skill

from .factories import make_skill, make_skill_version


class AuthGuardTests(TestCase):
    """Sin sesión autenticada → redirect a /login/ (no toca DB)."""

    def test_list_redirige_sin_auth(self):
        r = self.client.get(reverse("skills:list"))
        self.assertEqual(r.status_code, 302)
        self.assertIn("/login/", r["Location"])

    def test_signal_redirige_sin_auth(self):
        r = self.client.get(reverse("skills:signal"))
        self.assertEqual(r.status_code, 302)
        self.assertIn("/login/", r["Location"])


class ListViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()
        make_skill("Vista Skill", slug="vista", description="una skill")

    def test_list_ok_muestra_skill(self):
        r = self.client.get(reverse("skills:list"))
        self.assertEqual(r.status_code, 200)
        self.assertContains(r, "Vista Skill")

    def test_fragment_table_devuelve_solo_tabla(self):
        r = self.client.get(reverse("skills:list"), {"fragment": "table"})
        self.assertEqual(r.status_code, 200)
        body = r.content.decode()
        self.assertIn("<table", body)
        self.assertNotIn("<html", body)

    def test_search_filtra_server_side(self):
        make_skill("Otra Skill", slug="otra")
        r = self.client.get(reverse("skills:list"), {"q": "Vista", "fragment": "table"})
        self.assertContains(r, "Vista Skill")
        self.assertNotContains(r, "Otra Skill")


class SignalEndpointTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def test_signal_devuelve_json(self):
        make_skill("Sig", slug="sig")
        r = self.client.get(reverse("skills:signal"))
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
        s = make_skill("Detalle", slug="detalle")
        r = self.client.get(reverse("skills:detail", args=[s.pk]), {"partial": "1"})
        self.assertEqual(r.status_code, 200)
        body = r.content.decode()
        self.assertIn("modal-header", body)
        self.assertNotIn("<html", body)

    def test_detail_muestra_versiones(self):
        s = make_skill("Con Versiones", slug="con-versiones")
        make_skill_version(s, version=1, changelog="primera versión")
        r = self.client.get(reverse("skills:detail", args=[s.pk]), {"partial": "1"})
        self.assertContains(r, "primera versión")
        self.assertContains(r, "v1")

    def test_detail_inexistente_redirige(self):
        r = self.client.get(reverse("skills:detail", args=[uuid.uuid4()]))
        self.assertEqual(r.status_code, 302)


class CreateViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def _data(self, **over):
        base = {
            "name": "Creada",
            "slug": "creada",
            "skill_type": "prompt",
            "description": "una skill nueva",
            "content": "haz algo",
            "timeout_seconds": "30",
            "tags": "soporte, ventas",
        }
        base.update(over)
        return base

    def test_post_crea_skill(self):
        r = self.client.post(reverse("skills:create"), self._data())
        self.assertEqual(r.status_code, 302)
        self.assertTrue(Skill.objects.filter(slug="creada").exists())

    def test_post_tags_se_parsean(self):
        self.client.post(reverse("skills:create"), self._data(slug="contags"))
        s = Skill.objects.get(slug="contags")
        self.assertEqual(s.tags, ["soporte", "ventas"])

    def test_post_slug_duplicado_no_crea(self):
        make_skill("Existente", slug="dup")
        r = self.client.post(reverse("skills:create"), self._data(slug="dup"))
        # Form inválido (clean_slug) → re-render 200, sin crear nuevo.
        self.assertEqual(r.status_code, 200)
        self.assertEqual(Skill.objects.filter(slug="dup").count(), 1)

    def test_post_ajax_devuelve_redirect(self):
        r = self.client.post(reverse("skills:create"), self._data(slug="ajax"),
                             HTTP_X_REQUESTED_WITH="fetch")
        self.assertEqual(r.status_code, 302)
        self.assertTrue(Skill.objects.filter(slug="ajax").exists())


class EditViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def test_post_edita_skill(self):
        s = make_skill("Original", slug="orig", timeout_seconds=30)
        r = self.client.post(reverse("skills:edit", args=[s.pk]), {
            "name": "Editada",
            "slug": "orig",
            "skill_type": "prompt",
            "description": "",
            "content": "",
            "timeout_seconds": "120",
            "tags": "",
        })
        self.assertEqual(r.status_code, 302)
        s.refresh_from_db()
        self.assertEqual(s.name, "Editada")
        self.assertEqual(s.timeout_seconds, 120)


class DeleteViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()

    def test_post_soft_delete(self):
        s = make_skill("Del", slug="del")
        r = self.client.post(reverse("skills:delete", args=[s.pk]))
        self.assertEqual(r.status_code, 302)
        s.refresh_from_db()
        self.assertIsNotNone(s.deleted_at)
        self.assertTrue(Skill.objects.filter(pk=s.pk).exists())
