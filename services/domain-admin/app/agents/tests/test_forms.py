"""Tests de AgentForm (validaciones del mantenedor de Agentes).

Verifican reglas reales: campos requeridos, normalización de slug,
normalización de skills_slugs (CSV → lista), unicidad de slug
per-organización y exclusión del propio registro en edición.
"""
from __future__ import annotations

from django.test import TestCase

from agents.forms import AgentForm

from .factories import make_agent


class AgentFormCreateTests(TestCase):
    def _data(self, **over):
        base = {
            "name": "Form Bot",
            "slug": "form",
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

    def test_alta_valida(self):
        form = AgentForm(data=self._data())
        self.assertTrue(form.is_valid(), form.errors)

    def test_skills_slugs_se_normaliza_a_lista(self):
        form = AgentForm(data=self._data(skills_slugs="  a , b ,,c "))
        self.assertTrue(form.is_valid(), form.errors)
        self.assertEqual(form.cleaned_data["skills_slugs"], ["a", "b", "c"])

    def test_skills_slugs_vacio_da_lista_vacia(self):
        form = AgentForm(data=self._data(skills_slugs=""))
        self.assertTrue(form.is_valid(), form.errors)
        self.assertEqual(form.cleaned_data["skills_slugs"], [])

    def test_name_requerido(self):
        form = AgentForm(data=self._data(name=""))
        self.assertFalse(form.is_valid())
        self.assertIn("name", form.errors)

    def test_slug_requerido(self):
        form = AgentForm(data=self._data(slug=""))
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)

    def test_provider_requerido(self):
        form = AgentForm(data=self._data(provider=""))
        self.assertFalse(form.is_valid())
        self.assertIn("provider", form.errors)

    def test_model_requerido(self):
        form = AgentForm(data=self._data(model=""))
        self.assertFalse(form.is_valid())
        self.assertIn("model", form.errors)

    def test_slug_invalido_falla(self):
        # SlugField rechaza espacios.
        form = AgentForm(data=self._data(slug="con espacios"))
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)

    def test_max_iterations_no_numerico_falla(self):
        form = AgentForm(data=self._data(max_iterations="abc"))
        self.assertFalse(form.is_valid())
        self.assertIn("max_iterations", form.errors)

    def test_slug_duplicado(self):
        make_agent("Ocupado", slug="ocupado")
        form = AgentForm(data=self._data(slug="ocupado"))
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)


class AgentFormEditTests(TestCase):
    def _edit_data(self, a, **over):
        base = {
            "name": a.name,
            "slug": a.slug,
            "provider": a.provider,
            "model": a.model,
            "description": "",
            "system_prompt": "",
            "skills_slugs": "",
            "max_iterations": str(a.max_iterations),
            "token_budget": "",
            "temperature": "",
        }
        base.update(over)
        return base

    def test_edit_no_choca_con_su_propio_slug(self):
        a = make_agent("Mismo", slug="mismo")
        form = AgentForm(data=self._edit_data(a, name="Editado"), instance=a)
        self.assertTrue(form.is_valid(), form.errors)

    def test_edit_choca_con_slug_de_otro(self):
        make_agent("Otro", slug="otro")
        a = make_agent("Mio", slug="mio")
        form = AgentForm(data=self._edit_data(a, slug="otro"), instance=a)
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)
