"""Tests de PromptForm (validaciones del mantenedor de Prompts).

Verifican reglas reales: campos requeridos, normalizacion de slug, parseo de
tags, unicidad de la tripleta (project, slug, version) y exclusion del propio
registro en edicion (esta ultima delegada al core.forms.InstanceAwareMixin).
"""
from __future__ import annotations

from core.tests.base import MaintainerTestCase

from maintainers.prompts.forms import PromptForm

from .factories import make_prompt


class PromptFormCreateTests(MaintainerTestCase):
    def _data(self, **over):
        base = {
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
        # SlugField acepta "MiSlug"; clean_slug lo baja a minusculas.
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


class PromptFormEditTests(MaintainerTestCase):
    def _edit_data(self, p, **over):
        base = {
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
