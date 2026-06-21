"""Tests de validación de los forms del mantenedor de API Keys."""
from __future__ import annotations

from django.test import TestCase

from apikeys.forms import ApiKeyForm
from apikeys.models import ApiKey

from .factories import make_api_key, make_user


class ApiKeyFormCreateTests(TestCase):
    def setUp(self):
        self.owner = make_user("form@example.com")

    def test_valido(self):
        form = ApiKeyForm(data={
            "name": "Valida",
            "user": str(self.owner.pk),
            "status": "active",
        })
        self.assertTrue(form.is_valid(), form.errors)

    def test_user_obligatorio_en_create(self):
        form = ApiKeyForm(data={"name": "SinUser", "status": "active"})
        self.assertFalse(form.is_valid())
        self.assertIn("user", form.errors)

    def test_nombre_duplicado_invalido(self):
        make_api_key("Repetida", user=self.owner)
        form = ApiKeyForm(data={
            "name": "Repetida",
            "user": str(self.owner.pk),
            "status": "active",
        })
        self.assertFalse(form.is_valid())
        self.assertIn("name", form.errors)


class ApiKeyFormEditTests(TestCase):
    def test_edit_conserva_dueno_sin_user_en_post(self):
        ak = make_api_key("Editable")
        # En edición el select de user viene disabled (no llega en POST):
        # el form debe conservar el dueño original sin marcar error.
        form = ApiKeyForm(
            data={"name": "Editable v2", "status": "active"},
            instance=ak,
        )
        self.assertTrue(form.is_valid(), form.errors)
        self.assertEqual(form.cleaned_data["user"], str(ak.user.pk))

    def test_edit_mismo_nombre_no_choca_consigo_mismo(self):
        ak = make_api_key("MiNombre")
        form = ApiKeyForm(
            data={"name": "MiNombre", "status": "active"},
            instance=ak,
        )
        self.assertTrue(form.is_valid(), form.errors)


class ApiKeyFormInitialTests(TestCase):
    def test_initial_en_edicion(self):
        ak = make_api_key("Inicial")
        form = ApiKeyForm(instance=ak)
        self.assertEqual(form.fields["name"].initial, "Inicial")
        self.assertEqual(form.fields["status"].initial, ak.status)
