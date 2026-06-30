"""Tests de ProjectForm (validaciones del mantenedor).

Verifican reglas reales: slug requerido y normalizado, formato de slug y
unicidad global de slug. La captura de `instance` para excluirse en edicion se
delega a core.forms.InstanceAwareMixin. El campo `template` se quito del form
(su logica nunca se consumia) y los repos git ya no son fields (van como filas
dinamicas que parsea la view), por eso no aparecen aqui.
"""
from __future__ import annotations

from core.tests.base import MaintainerTestCase

from maintainers.projects.forms import ProjectForm

from .factories import make_project


class ProjectFormCreateTests(MaintainerTestCase):
    def _data(self, **over):
        base = {
            "name": "Form",
            "slug": "form",
            "description": "",
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

    def test_slug_duplicado(self):
        make_project("Ocupado", slug="ocupado")
        form = ProjectForm(data=self._data(slug="ocupado"))
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)


class ProjectFormEditTests(MaintainerTestCase):
    def test_edit_mantiene_su_propio_slug(self):
        p = make_project("Mismo", slug="mismo")
        form = ProjectForm(
            data={
                "name": "Mismo v2",
                "slug": "mismo",
                "description": "",
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
                "name": "Mio",
                "slug": "otro",
                "description": "",
                "current_branch": "",
            },
            instance=p,
        )
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)
