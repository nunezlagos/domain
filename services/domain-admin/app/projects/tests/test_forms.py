"""Tests de ProjectForm (validaciones del mantenedor).

Verifican reglas reales: slug requerido y normalizado, formato de slug,
unicidad de slug per-organización y selección de template.
"""
from __future__ import annotations

from django.test import TestCase

from projects.forms import ProjectForm

from .factories import DEFAULT_ORG, make_project, make_template


class ProjectFormCreateTests(TestCase):
    def _data(self, **over):
        base = {
            "organization_id": str(DEFAULT_ORG),
            "name": "Form",
            "slug": "form",
            "description": "",
            "repository_url": "",
            "template": "",
            "current_branch": "",
        }
        base.update(over)
        return base

    def test_alta_valida(self):
        form = ProjectForm(data=self._data())
        self.assertTrue(form.is_valid(), form.errors)

    def test_slug_se_normaliza_minuscula(self):
        form = ProjectForm(data=self._data(slug="MiSlug"))
        self.assertTrue(form.is_valid(), form.errors)
        self.assertEqual(form.cleaned_data["slug"], "mislug")

    def test_slug_con_espacios_invalido(self):
        form = ProjectForm(data=self._data(slug="con espacios"))
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)

    def test_slug_con_simbolos_invalido(self):
        form = ProjectForm(data=self._data(slug="slug_invalido!"))
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)

    def test_slug_duplicado_misma_org(self):
        make_project("Ocupado", slug="ocupado")
        form = ProjectForm(data=self._data(slug="ocupado"))
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)

    def test_template_vacio_es_none(self):
        form = ProjectForm(data=self._data(template=""))
        self.assertTrue(form.is_valid(), form.errors)
        self.assertIsNone(form.cleaned_data["template"])

    def test_template_valido_se_acepta(self):
        tpl = make_template("django")
        form = ProjectForm(data=self._data(template=str(tpl.pk)))
        self.assertTrue(form.is_valid(), form.errors)
        self.assertEqual(form.cleaned_data["template"], str(tpl.pk))


class ProjectFormEditTests(TestCase):
    def test_edit_mantiene_su_propio_slug(self):
        p = make_project("Mismo", slug="mismo")
        form = ProjectForm(
            data={
                "organization_id": str(p.organization_id),
                "name": "Mismo v2",
                "slug": "mismo",
                "description": "",
                "repository_url": "",
                "template": "",
                "current_branch": "",
            },
            instance=p,
        )
        self.assertTrue(form.is_valid(), form.errors)

    def test_edit_choca_con_otro_slug(self):
        make_project("Otro", slug="otro")
        p = make_project("Mio", slug="mio")
        form = ProjectForm(
            data={
                "organization_id": str(p.organization_id),
                "name": "Mio",
                "slug": "otro",
                "description": "",
                "repository_url": "",
                "template": "",
                "current_branch": "",
            },
            instance=p,
        )
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)
