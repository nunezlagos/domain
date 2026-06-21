"""Tests de las views (HTTP) del mantenedor de Proyectos.

Usan el test client real contra URLs reales. Verifican status codes, efectos
en DB y forma de la respuesta (HTML vs JSON vs partial).
"""
from __future__ import annotations

import json

from django.test import TestCase
from django.urls import reverse

from projects.models import Project

from .factories import make_project


class AuthGuardTests(TestCase):
    """Sin sesión autenticada → redirect a /login/ (no toca DB)."""

    def test_list_redirige_sin_auth(self):
        r = self.client.get(reverse("projects:list"))
        self.assertEqual(r.status_code, 302)
        self.assertIn("/login/", r["Location"])

    def test_signal_redirige_sin_auth(self):
        r = self.client.get(reverse("projects:signal"))
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
        make_project("Vista", slug="vista")

    def test_list_ok_muestra_proyecto(self):
        r = self.client.get(reverse("projects:list"))
        self.assertEqual(r.status_code, 200)
        self.assertContains(r, "vista")

    def test_fragment_table_devuelve_solo_tabla(self):
        r = self.client.get(reverse("projects:list"), {"fragment": "table"})
        self.assertEqual(r.status_code, 200)
        body = r.content.decode()
        self.assertIn("<table", body)
        self.assertNotIn("<html", body)

    def test_search_filtra_server_side(self):
        make_project("Otro", slug="otro")
        r = self.client.get(reverse("projects:list"), {"q": "vista", "fragment": "table"})
        self.assertContains(r, "vista")
        self.assertNotContains(r, "otro")


class SignalEndpointTests(AuthenticatedMixin, TestCase):
    def setUp(self):
        self.authenticate()

    def test_signal_devuelve_json(self):
        make_project("Sig", slug="sig")
        r = self.client.get(reverse("projects:signal"))
        self.assertEqual(r.status_code, 200)
        self.assertEqual(r["Content-Type"], "application/json")
        data = json.loads(r.content)
        self.assertIn("count", data)
        self.assertIn("version", data)
        self.assertEqual(data["count"], 1)


class DetailViewTests(AuthenticatedMixin, TestCase):
    def setUp(self):
        self.authenticate()

    def test_detail_partial_es_modal(self):
        p = make_project("Det", slug="det")
        r = self.client.get(reverse("projects:detail", args=[p.pk]), {"partial": "1"})
        self.assertEqual(r.status_code, 200)
        body = r.content.decode()
        self.assertIn("modal-header", body)
        self.assertNotIn("<html", body)

    def test_detail_inexistente_redirige(self):
        import uuid
        r = self.client.get(reverse("projects:detail", args=[uuid.uuid4()]))
        self.assertEqual(r.status_code, 302)


class CreateViewTests(AuthenticatedMixin, TestCase):
    def setUp(self):
        self.authenticate()

    def _data(self, **over):
        base = {
            "name": "Creado",
            "slug": "creado",
            "description": "",
            "repository_url": "",
            "template": "",
            "current_branch": "",
        }
        base.update(over)
        return base

    def test_post_crea_proyecto(self):
        r = self.client.post(reverse("projects:create"), self._data())
        self.assertEqual(r.status_code, 302)
        self.assertTrue(Project.objects.filter(slug="creado").exists())

    def test_post_ajax_redirige_a_list(self):
        r = self.client.post(reverse("projects:create"), self._data(slug="ajax"),
                             HTTP_X_REQUESTED_WITH="fetch")
        self.assertEqual(r.status_code, 302)
        self.assertIn(reverse("projects:list"), r["Location"])

    def test_post_slug_invalido_no_crea(self):
        r = self.client.post(reverse("projects:create"), self._data(slug="Con Espacios"))
        self.assertEqual(r.status_code, 200)  # re-render del form
        self.assertFalse(Project.objects.filter(name="Creado").exists())

    def test_get_partial_devuelve_form_modal(self):
        r = self.client.get(reverse("projects:create"), {"partial": "1"})
        self.assertEqual(r.status_code, 200)
        body = r.content.decode()
        self.assertIn("data-modal-form", body)
        self.assertNotIn("<html", body)


class EditViewTests(AuthenticatedMixin, TestCase):
    def setUp(self):
        self.authenticate()

    def test_post_edita_proyecto(self):
        p = make_project("Edit", slug="edit")
        r = self.client.post(reverse("projects:edit", args=[p.pk]), {
            "name": "Editado",
            "slug": "edit",
            "description": "",
            "repository_url": "",
            "template": "",
            "current_branch": "main",
        })
        self.assertEqual(r.status_code, 302)
        p.refresh_from_db()
        self.assertEqual(p.name, "Editado")
        self.assertEqual(p.current_branch, "main")


class ToggleViewTests(AuthenticatedMixin, TestCase):
    def setUp(self):
        self.authenticate()

    def test_post_toggle_archiva(self):
        p = make_project("Tog", slug="tog")
        r = self.client.post(reverse("projects:toggle", args=[p.pk]),
                             HTTP_X_REQUESTED_WITH="fetch")
        self.assertEqual(r.status_code, 302)
        p.refresh_from_db()
        self.assertIsNotNone(p.deleted_at)

    def test_post_toggle_restaura(self):
        p = make_project("Tog", slug="tog", archived=True)
        r = self.client.post(reverse("projects:toggle", args=[p.pk]),
                             HTTP_X_REQUESTED_WITH="fetch")
        self.assertEqual(r.status_code, 302)
        p.refresh_from_db()
        self.assertIsNone(p.deleted_at)


class DeleteViewTests(AuthenticatedMixin, TestCase):
    def setUp(self):
        self.authenticate()

    def test_post_soft_delete(self):
        p = make_project("Del", slug="del")
        r = self.client.post(reverse("projects:delete", args=[p.pk]))
        self.assertEqual(r.status_code, 302)
        p.refresh_from_db()
        self.assertIsNotNone(p.deleted_at)
        self.assertTrue(Project.objects.filter(pk=p.pk).exists())
