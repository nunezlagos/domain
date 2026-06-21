"""Tests de UserForm (validaciones del mantenedor).

Verifican reglas reales: password requerido en alta, confirmación,
longitud mínima, email único y normalización a minúsculas.
"""
from __future__ import annotations

from django.test import TestCase

from users.forms import UserForm

from .factories import make_role, make_user


class UserFormCreateTests(TestCase):
    def setUp(self):
        make_role("viewer")

    def _data(self, **over):
        base = {
            "email": "form@example.com",
            "name": "Form",
            "role": "viewer",
            "status": "active",
            "password": "supersecret",
            "password_confirm": "supersecret",
        }
        base.update(over)
        return base

    def test_alta_valida(self):
        form = UserForm(data=self._data())
        self.assertTrue(form.is_valid(), form.errors)

    def test_password_requerido_en_alta(self):
        form = UserForm(data=self._data(password="", password_confirm=""))
        self.assertFalse(form.is_valid())
        self.assertIn("password", form.errors)

    def test_passwords_no_coinciden(self):
        form = UserForm(data=self._data(password_confirm="otracosa"))
        self.assertFalse(form.is_valid())

    def test_password_corta(self):
        form = UserForm(data=self._data(password="abc", password_confirm="abc"))
        self.assertFalse(form.is_valid())

    def test_email_se_normaliza_minuscula(self):
        form = UserForm(data=self._data(email="MAYUS@Example.COM"))
        self.assertTrue(form.is_valid(), form.errors)
        self.assertEqual(form.cleaned_data["email"], "mayus@example.com")

    def test_email_duplicado(self):
        make_user("ocupado@example.com")
        form = UserForm(data=self._data(email="ocupado@example.com"))
        self.assertFalse(form.is_valid())
        self.assertIn("email", form.errors)


class UserFormEditTests(TestCase):
    def setUp(self):
        make_role("viewer")

    def test_edit_password_vacio_es_valido(self):
        u = make_user("edit@example.com")
        form = UserForm(
            data={
                "email": "edit@example.com",
                "name": "Editado",
                "role": "viewer",
                "status": "active",
                "password": "",
                "password_confirm": "",
            },
            instance=u,
        )
        self.assertTrue(form.is_valid(), form.errors)

    def test_edit_no_choca_con_su_propio_email(self):
        u = make_user("mismo@example.com")
        form = UserForm(
            data={
                "email": "mismo@example.com",
                "name": "X",
                "role": "viewer",
                "status": "active",
                "password": "",
                "password_confirm": "",
            },
            instance=u,
        )
        self.assertTrue(form.is_valid(), form.errors)
