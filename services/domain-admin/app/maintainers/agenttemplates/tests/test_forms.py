"""Tests de AgentTemplateForm (validaciones del mantenedor de Plantillas).

Verifican reglas reales: campos requeridos, normalización de slug, parseo de
capabilities, rangos (temperature/max_tokens), choices y unicidad de slug
(global) con exclusión del propio registro en edición.
"""
from __future__ import annotations

from core.tests.base import MaintainerTestCase

from maintainers.agenttemplates.forms import AgentTemplateForm

from .factories import make_agent_template


class AgentTemplateFormCreateTests(MaintainerTestCase):
    def _data(self, **over):
        base = {
            "name": "Form Plantilla",
            "slug": "form",
            "system_prompt": "Sos útil.",
            "personality": "amable",
            "capabilities": "research, code",
            "model": "claude-haiku-4-5",
            "temperature": "0.7",
            "max_tokens": "4096",
            "handoff_policy": "allow",
            "role": "phase-worker",
        }
        base.update(over)
        return base

    def test_alta_valida(self):
        form = AgentTemplateForm(data=self._data())
        self.assertTrue(form.is_valid(), form.errors)

    def test_name_requerido(self):
        form = AgentTemplateForm(data=self._data(name=""))
        self.assertFalse(form.is_valid())
        self.assertIn("name", form.errors)

    def test_slug_requerido(self):
        form = AgentTemplateForm(data=self._data(slug=""))
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)

    def test_system_prompt_requerido(self):
        form = AgentTemplateForm(data=self._data(system_prompt=""))
        self.assertFalse(form.is_valid())
        self.assertIn("system_prompt", form.errors)

    def test_slug_invalido_falla(self):
        form = AgentTemplateForm(data=self._data(slug="con espacios"))
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)

    def test_temperature_fuera_de_rango_falla(self):
        form = AgentTemplateForm(data=self._data(temperature="3"))
        self.assertFalse(form.is_valid())
        self.assertIn("temperature", form.errors)

    def test_max_tokens_fuera_de_rango_falla(self):
        form = AgentTemplateForm(data=self._data(max_tokens="0"))
        self.assertFalse(form.is_valid())
        self.assertIn("max_tokens", form.errors)

    def test_role_invalido_falla(self):
        form = AgentTemplateForm(data=self._data(role="no-existe"))
        self.assertFalse(form.is_valid())
        self.assertIn("role", form.errors)

    def test_handoff_policy_invalido_falla(self):
        form = AgentTemplateForm(data=self._data(handoff_policy="no-existe"))
        self.assertFalse(form.is_valid())
        self.assertIn("handoff_policy", form.errors)

    def test_capabilities_se_parsean_a_lista(self):
        form = AgentTemplateForm(data=self._data())
        self.assertTrue(form.is_valid(), form.errors)
        self.assertEqual(form.cleaned_data["capabilities"], ["research", "code"])

    def test_capabilities_vacias_es_lista_vacia(self):
        form = AgentTemplateForm(data=self._data(capabilities=""))
        self.assertTrue(form.is_valid(), form.errors)
        self.assertEqual(form.cleaned_data["capabilities"], [])

    def test_slug_se_normaliza_a_minusculas(self):
        form = AgentTemplateForm(data=self._data(slug="MiPlantilla"))
        self.assertTrue(form.is_valid(), form.errors)
        self.assertEqual(form.cleaned_data["slug"], "miplantilla")

    def test_slug_duplicado_falla(self):
        make_agent_template("Ocupado", slug="ocupado")
        form = AgentTemplateForm(data=self._data(slug="ocupado"))
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)


class AgentTemplateFormEditTests(MaintainerTestCase):
    def _edit_data(self, t, **over):
        base = {
            "name": t.name,
            "slug": t.slug,
            "system_prompt": t.system_prompt,
            "personality": "",
            "capabilities": "",
            "model": t.model,
            "temperature": str(t.temperature),
            "max_tokens": str(t.max_tokens),
            "handoff_policy": t.handoff_policy,
            "role": t.role,
        }
        base.update(over)
        return base

    def test_edit_no_choca_con_su_propio_slug(self):
        t = make_agent_template("Mismo", slug="mismo")
        form = AgentTemplateForm(data=self._edit_data(t, name="Editada"), instance=t)
        self.assertTrue(form.is_valid(), form.errors)

    def test_edit_choca_con_slug_de_otro(self):
        make_agent_template("Otra", slug="otra")
        t = make_agent_template("Mia", slug="mia")
        form = AgentTemplateForm(data=self._edit_data(t, slug="otra"), instance=t)
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)
