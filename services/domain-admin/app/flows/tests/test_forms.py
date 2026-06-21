"""Tests de FlowForm (validaciones del mantenedor de Flows).

Verifican reglas reales: campos requeridos, JSON del spec, normalización
de slug, unicidad de slug y exclusión del propio registro en edición.
"""
from __future__ import annotations

from django.test import TestCase

from flows.forms import FlowForm

from .factories import make_flow


class FlowFormCreateTests(TestCase):
    def _data(self, **over):
        base = {
            "name": "Form Flow",
            "slug": "form",
            "description": "",
            "spec": "{}",
            "is_active": "on",
            "seed_version": "",
        }
        base.update(over)
        return base

    def test_alta_valida(self):
        form = FlowForm(data=self._data())
        self.assertTrue(form.is_valid(), form.errors)

    def test_spec_parsea_a_dict(self):
        form = FlowForm(data=self._data(spec='{"steps": [1, 2]}'))
        self.assertTrue(form.is_valid(), form.errors)
        self.assertEqual(form.cleaned_data["spec"], {"steps": [1, 2]})

    def test_is_active_marcado_es_true(self):
        form = FlowForm(data=self._data(is_active="on"))
        self.assertTrue(form.is_valid(), form.errors)
        self.assertTrue(form.cleaned_data["is_active"])

    def test_is_active_omitido_es_false(self):
        data = self._data()
        data.pop("is_active")
        form = FlowForm(data=data)
        self.assertTrue(form.is_valid(), form.errors)
        self.assertFalse(form.cleaned_data["is_active"])

    def test_name_requerido(self):
        form = FlowForm(data=self._data(name=""))
        self.assertFalse(form.is_valid())
        self.assertIn("name", form.errors)

    def test_slug_requerido(self):
        form = FlowForm(data=self._data(slug=""))
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)

    def test_spec_invalido_falla(self):
        form = FlowForm(data=self._data(spec="no-es-json"))
        self.assertFalse(form.is_valid())
        self.assertIn("spec", form.errors)

    def test_slug_invalido_falla(self):
        form = FlowForm(data=self._data(slug="con espacios"))
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)

    def test_slug_duplicado(self):
        make_flow("Ocupado", slug="ocupado")
        form = FlowForm(data=self._data(slug="ocupado"))
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)


class FlowFormEditTests(TestCase):
    def _edit_data(self, f, **over):
        base = {
            "name": f.name,
            "slug": f.slug,
            "description": "",
            "spec": "{}",
            "is_active": "on",
            "seed_version": "",
        }
        base.update(over)
        return base

    def test_edit_no_choca_con_su_propio_slug(self):
        f = make_flow("Mismo", slug="mismo")
        form = FlowForm(data=self._edit_data(f, name="Editado"), instance=f)
        self.assertTrue(form.is_valid(), form.errors)

    def test_edit_choca_con_slug_de_otro(self):
        make_flow("Otro", slug="otro")
        f = make_flow("Mio", slug="mio")
        form = FlowForm(data=self._edit_data(f, slug="otro"), instance=f)
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)
