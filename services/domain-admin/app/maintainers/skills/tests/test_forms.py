"""Tests de SkillForm (validaciones del mantenedor de Skills).

Verifican reglas reales: campos requeridos, normalizacion de slug, parseo de
tags, rango de timeout, unicidad de slug per-scope y exclusion del propio
registro en edicion.
"""
from __future__ import annotations

from core.tests.base import MaintainerTestCase

from maintainers.skills.forms import SkillForm

from .factories import make_skill


class SkillFormCreateTests(MaintainerTestCase):
    def _data(self, **over):
        base = {
            "name": "Form Skill",
            "slug": "form",
            "skill_type": "prompt",
            "description": "una skill",
            "content": "cuerpo",
            "timeout_seconds": "30",
            "tags": "soporte, ventas",
        }
        base.update(over)
        return base

    def test_alta_valida(self):
        form = SkillForm(data=self._data())
        self.assertTrue(form.is_valid(), form.errors)

    def test_name_requerido(self):
        form = SkillForm(data=self._data(name=""))
        self.assertFalse(form.is_valid())
        self.assertIn("name", form.errors)

    def test_slug_requerido(self):
        form = SkillForm(data=self._data(slug=""))
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)

    def test_slug_invalido_falla(self):
        form = SkillForm(data=self._data(slug="con espacios"))
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)

    def test_timeout_fuera_de_rango_falla(self):
        form = SkillForm(data=self._data(timeout_seconds="0"))
        self.assertFalse(form.is_valid())
        self.assertIn("timeout_seconds", form.errors)

        form2 = SkillForm(data=self._data(timeout_seconds="601"))
        self.assertFalse(form2.is_valid())
        self.assertIn("timeout_seconds", form2.errors)

    def test_skill_type_invalido_falla(self):
        form = SkillForm(data=self._data(skill_type="no-existe"))
        self.assertFalse(form.is_valid())
        self.assertIn("skill_type", form.errors)

    def test_tags_se_parsean_a_lista(self):
        form = SkillForm(data=self._data())
        self.assertTrue(form.is_valid(), form.errors)
        self.assertEqual(form.cleaned_data["tags"], ["soporte", "ventas"])

    def test_tags_vacios_es_lista_vacia(self):
        form = SkillForm(data=self._data(tags=""))
        self.assertTrue(form.is_valid(), form.errors)
        self.assertEqual(form.cleaned_data["tags"], [])

    def test_slug_se_normaliza_a_minusculas(self):
        form = SkillForm(data=self._data(slug="MiSkill"))

        self.assertTrue(form.is_valid(), form.errors)
        self.assertEqual(form.cleaned_data["slug"], "miskill")

    def test_slug_duplicado_en_mismo_scope(self):
        make_skill("Ocupado", slug="ocupado")  # global
        form = SkillForm(data=self._data(slug="ocupado"))
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)


class SkillFormEditTests(MaintainerTestCase):
    def _edit_data(self, s, **over):
        base = {
            "name": s.name,
            "slug": s.slug,
            "skill_type": s.skill_type,
            "description": "",
            "content": "",
            "timeout_seconds": str(s.timeout_seconds),
            "tags": "",
        }
        base.update(over)
        return base

    def test_edit_no_choca_con_su_propio_slug(self):
        s = make_skill("Mismo", slug="mismo")
        form = SkillForm(data=self._edit_data(s, name="Editada"), instance=s)
        self.assertTrue(form.is_valid(), form.errors)

    def test_edit_choca_con_slug_de_otro(self):
        make_skill("Otra", slug="otra")
        s = make_skill("Mia", slug="mia")
        form = SkillForm(data=self._edit_data(s, slug="otra"), instance=s)
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)
