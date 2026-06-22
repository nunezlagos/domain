"""Tests HTTP del mantenedor de Reglas por proyecto."""
from __future__ import annotations

import json

from django.test import TestCase
from django.urls import reverse

from core.tests.base import MaintainerTestCase

from maintainers.projectpolicies.models import ProjectPolicy
from maintainers.projects.tests.factories import make_project

from .factories import make_policy


class AuthGuardTests(TestCase):
    def test_list_redirige_sin_auth(self):
        r = self.client.get(reverse("projectpolicies:list"))
        self.assertEqual(r.status_code, 302)
        self.assertIn("/login/", r["Location"])


class ListViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()
        make_policy("Regla Vista", slug="regla-vista")

    def test_list_ok(self):
        r = self.client.get(reverse("projectpolicies:list"))
        self.assertEqual(r.status_code, 200)
        self.assertContains(r, "regla-vista")

    def test_signal_json(self):
        r = self.client.get(reverse("projectpolicies:signal"))
        self.assertEqual(r.status_code, 200)
        data = json.loads(r.content)
        self.assertIn("count", data)


class CreateViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()
        self.project = make_project("Demo", slug="demo")

    def test_post_crea_regla(self):
        r = self.client.post(reverse("projectpolicies:create"), {
            "project": str(self.project.pk),
            "name": "Commits en español",
            "slug": "commits",
            "kind": "convention",
            "body_md": "Commits en español.",
            "is_active": "on",
        })
        self.assertEqual(r.status_code, 302)
        p = ProjectPolicy.objects.get(slug="commits")
        self.assertEqual(p.project_id, self.project.pk)
        self.assertEqual(p.source, "dashboard")

    def test_get_partial_form(self):
        r = self.client.get(reverse("projectpolicies:create"), {"partial": "1"})
        self.assertEqual(r.status_code, 200)
        self.assertIn("data-modal-form", r.content.decode())
