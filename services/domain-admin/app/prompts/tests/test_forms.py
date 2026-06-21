"""Tests de PromptForm (validaciones del mantenedor de Prompts).

Verifican reglas reales: campos requeridos, normalización de slug, parseo
de tags, unicidad de la cuádrupla (org, project, slug, version) y exclusión
del propio registro en edición.
"""
from __future__ import annotations

from django.test import TestCase

from prompts.forms import PromptForm

from .factories import DEFAULT_ORG, make_prompt


class PromptFormCreateTests(TestCase):
    def _data(self, **over):
        base = {
            "organization_id": str(DEFAULT_ORG),
            "project_id": "",
            "slug": "form",
            "version": "1",
            "body": "Cuerpo del prompt.",
            "description": "desc",
            "tags": "soporte, ventas",
            "is_active": "on",
        }
        base.update(over)
        return base

    def test_alta_valida(self):
        form = PromptForm(data=self._data())
        self.assertTrue(form.is_valid(), form.errors)

    def test_slug_requerido(self):
        form = PromptForm(data=self._data(slug=""))
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)

    def test_body_requerido(self):
        form = PromptForm(data=self._data(body=""))
        self.assertFalse(form.is_valid())
        self.assertIn("body", form.errors)

    def test_organization_id_requerido_en_alta(self):
        form = PromptForm(data=self._data(organization_id=""))
        self.assertFalse(form.is_valid())
        self.assertIn("organization_id", form.errors)

    def test_slug_invalido_falla(self):
        form = PromptForm(data=self._data(slug="con espacios"))
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)

    def test_version_invalida_falla(self):
        form = PromptForm(data=self._data(version="0"))
        self.assertFalse(form.is_valid())
        self.assertIn("version", form.errors)

    def test_tags_se_parsean_a_lista(self):
        form = PromptForm(data=self._data(tags=" a , b ,, c "))
        self.assertTrue(form.is_valid(), form.errors)
        self.assertEqual(form.cleaned_data["tags"], ["a", "b", "c"])

    def test_slug_se_normaliza_minuscula(self):
        form = PromptForm(data=self._data(slug="MiSlug"))
        # SlugField ya rechaza mayúsculas con espacios, pero "MiSlug" es válido
        # como slug; clean_slug lo baja a minúsculas.
        self.assertTrue(form.is_valid(), form.errors)
        self.assertEqual(form.cleaned_data["slug"], "mislug")

    def test_cuadrupla_duplicada_falla(self):
        make_prompt("ocupado", version=1)
        form = PromptForm(data=self._data(slug="ocupado", version="1"))
        self.assertFalse(form.is_valid())
        self.assertIn("__all__", form.errors)

    def test_mismo_slug_otra_version_ok(self):
        make_prompt("verx", version=1)
        form = PromptForm(data=self._data(slug="verx", version="2"))
        self.assertTrue(form.is_valid(), form.errors)


class PromptFormEditTests(TestCase):
    def _edit_data(self, p, **over):
        base = {
            "organization_id": str(p.organization_id),
            "project_id": "",
            "slug": p.slug,
            "version": str(p.version),
            "body": p.body,
            "description": "",
            "tags": "",
            "is_active": "on" if p.is_active else "",
        }
        base.update(over)
        return base

    def test_edit_no_choca_con_su_propia_cuadrupla(self):
        p = make_prompt("mismo", version=1)
        form = PromptForm(data=self._edit_data(p, body="editado"), instance=p)
        self.assertTrue(form.is_valid(), form.errors)

    def test_edit_choca_con_cuadrupla_de_otro(self):
        make_prompt("otro", version=2)
        p = make_prompt("otro", version=1)
        form = PromptForm(data=self._edit_data(p, version="2"), instance=p)
        self.assertFalse(form.is_valid())
        self.assertIn("__all__", form.errors)
