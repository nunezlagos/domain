"""Tests de ProjectPolicyForm."""
from __future__ import annotations

from core.tests.base import MaintainerTestCase

from maintainers.projectpolicies.forms import ProjectPolicyForm
from maintainers.projects.tests.factories import make_project

from .factories import make_policy


class ProjectPolicyFormTests(MaintainerTestCase):
    def setUp(self):
        self.project = make_project("Demo", slug="demo")

    def _data(self, **over):
        base = {
            "project": str(self.project.pk),
            "name": "Commits en español",
            "slug": "commits",
            "kind": "convention",
            "body_md": "Los commits se escriben en español.",
            "override_platform": "",
            "is_active": "on",
        }
        base.update(over)
        return base

    def test_alta_valida(self):
        form = ProjectPolicyForm(data=self._data())
        self.assertTrue(form.is_valid(), form.errors)

    def test_slug_se_normaliza(self):
        form = ProjectPolicyForm(data=self._data(slug="MiRegla"))
        self.assertTrue(form.is_valid(), form.errors)
        self.assertEqual(form.cleaned_data["slug"], "miregla")

    def test_kind_invalido(self):
        form = ProjectPolicyForm(data=self._data(kind="inexistente"))
        self.assertFalse(form.is_valid())
        self.assertIn("kind", form.errors)

    def test_slug_duplicado_en_proyecto(self):
        make_policy("X", project_id=self.project.pk, slug="dup")
        form = ProjectPolicyForm(data=self._data(slug="dup"))
        self.assertFalse(form.is_valid())
