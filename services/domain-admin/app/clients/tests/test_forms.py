"""Tests de ClientForm (validaciones del mantenedor de Clientes).

Verifican reglas reales: campos requeridos, normalización de slug,
unicidad de slug per-organización y exclusión del propio registro en edición.
"""
from __future__ import annotations

from django.test import TestCase

from clients.forms import ClientForm

from .factories import DEFAULT_ORG, make_client


class ClientFormCreateTests(TestCase):
    def _data(self, **over):
        base = {
            "organization_id": str(DEFAULT_ORG),
            "name": "Form SpA",
            "slug": "form",
            "tax_id": "",
            "contact_email": "ops@form.com",
            "contact_phone": "",
            "address": "",
            "status": "active",
        }
        base.update(over)
        return base

    def test_alta_valida(self):
        form = ClientForm(data=self._data())
        self.assertTrue(form.is_valid(), form.errors)

    def test_name_requerido(self):
        form = ClientForm(data=self._data(name=""))
        self.assertFalse(form.is_valid())
        self.assertIn("name", form.errors)

    def test_slug_requerido(self):
        form = ClientForm(data=self._data(slug=""))
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)

    def test_organization_id_requerido_en_alta(self):
        form = ClientForm(data=self._data(organization_id=""))
        self.assertFalse(form.is_valid())
        self.assertIn("organization_id", form.errors)

    def test_slug_invalido_falla(self):
        # SlugField rechaza espacios y mayúsculas con caracteres no válidos.
        form = ClientForm(data=self._data(slug="con espacios"))
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)

    def test_email_invalido_falla(self):
        form = ClientForm(data=self._data(contact_email="no-es-email"))
        self.assertFalse(form.is_valid())
        self.assertIn("contact_email", form.errors)

    def test_slug_duplicado_en_misma_org(self):
        make_client("Ocupado", slug="ocupado")
        form = ClientForm(data=self._data(slug="ocupado"))
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)


class ClientFormEditTests(TestCase):
    def _edit_data(self, c, **over):
        base = {
            "organization_id": str(c.organization_id),
            "name": c.name,
            "slug": c.slug,
            "tax_id": "",
            "contact_email": "",
            "contact_phone": "",
            "address": "",
            "status": c.status,
        }
        base.update(over)
        return base

    def test_edit_no_choca_con_su_propio_slug(self):
        c = make_client("Mismo", slug="mismo")
        form = ClientForm(data=self._edit_data(c, name="Editado"), instance=c)
        self.assertTrue(form.is_valid(), form.errors)

    def test_edit_choca_con_slug_de_otro(self):
        make_client("Otro", slug="otro")
        c = make_client("Mio", slug="mio")
        form = ClientForm(data=self._edit_data(c, slug="otro"), instance=c)
        self.assertFalse(form.is_valid())
        self.assertIn("slug", form.errors)
