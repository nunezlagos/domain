"""Tests de CronForm (validaciones del mantenedor de Crons).

Verifican reglas reales: campos requeridos, normalización de slug, parseo de
inputs JSON, unicidad de slug y exclusión del propio registro en edición (esta
última delegada a core.forms.InstanceAwareMixin).
"""
from __future__ import annotations

from core.tests.base import MaintainerTestCase

from maintainers.crons.forms import CronForm

from .factories import DEFAULT_TARGET, make_cron


class CronFormCreateTests(MaintainerTestCase):
    def _data(self, **over):
        base = {
            "name": "Form Cron",
            "slug": "form",
            "description": "",
            "cron_expression": "0 9 * * *",
            "timezone": "UTC",
            "target_type": "flow",
            "target_id": str(DEFAULT_TARGET),
            "inputs": "{}",
            "enabled": "on",
        }
        base.update(over)
        return base

    def test_alta_valida(self):
        form = CronForm(data=self._data())
        self.assertTrue(form.is_valid(), form.errors)

    def test_name_requerido(self):
        form = CronForm(data=self._data(name=""))
        self.assertFalse(form.is_valid())
        self.assertIn("name", form.errors)

    def test_slug_requerido(self):
        form = CronForm(data=self._data(slug=""))
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)

    def test_cron_expression_requerido(self):
        form = CronForm(data=self._data(cron_expression=""))
        self.assertFalse(form.is_valid())
        self.assertIn("cron_expression", form.errors)

    def test_target_id_requerido(self):
        form = CronForm(data=self._data(target_id=""))
        self.assertFalse(form.is_valid())
        self.assertIn("target_id", form.errors)

    def test_slug_invalido_falla(self):
        form = CronForm(data=self._data(slug="con espacios"))
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)

    def test_target_type_invalido_falla(self):
        form = CronForm(data=self._data(target_type="webhook"))
        self.assertFalse(form.is_valid())
        self.assertIn("target_type", form.errors)

    def test_inputs_json_invalido_falla(self):
        form = CronForm(data=self._data(inputs="no-es-json"))
        self.assertFalse(form.is_valid())
        self.assertIn("inputs", form.errors)

    def test_inputs_no_objeto_falla(self):
        form = CronForm(data=self._data(inputs="[1, 2, 3]"))
        self.assertFalse(form.is_valid())
        self.assertIn("inputs", form.errors)

    def test_inputs_vacio_se_normaliza_a_dict(self):
        form = CronForm(data=self._data(inputs=""))
        self.assertTrue(form.is_valid(), form.errors)
        self.assertEqual(form.cleaned_data["inputs"], {})

    def test_timezone_vacio_default_utc(self):
        form = CronForm(data=self._data(timezone=""))
        self.assertTrue(form.is_valid(), form.errors)
        self.assertEqual(form.cleaned_data["timezone"], "UTC")

    def test_slug_duplicado(self):
        make_cron("Ocupado", slug="ocupado")
        form = CronForm(data=self._data(slug="ocupado"))
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)


class CronFormEditTests(MaintainerTestCase):
    def _edit_data(self, c, **over):
        base = {
            "name": c.name,
            "slug": c.slug,
            "description": "",
            "cron_expression": c.cron_expression,
            "timezone": c.timezone,
            "target_type": c.target_type,
            "target_id": str(c.target_id),
            "inputs": "{}",
            "enabled": "on",
        }
        base.update(over)
        return base

    def test_edit_no_choca_con_su_propio_slug(self):
        c = make_cron("Mismo", slug="mismo")
        form = CronForm(data=self._edit_data(c, name="Editado"), instance=c)
        self.assertTrue(form.is_valid(), form.errors)

    def test_edit_choca_con_slug_de_otro(self):
        make_cron("Otro", slug="otro")
        c = make_cron("Mio", slug="mio")
        form = CronForm(data=self._edit_data(c, slug="otro"), instance=c)
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)
