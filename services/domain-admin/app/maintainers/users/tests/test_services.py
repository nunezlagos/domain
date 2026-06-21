"""Tests de la capa de servicio del mantenedor de usuarios.

Corren contra Postgres real (managed=True via core.tests.runner). Cada assert
verifica un efecto observable en la DB o el valor de retorno real, no un mock.
"""
from __future__ import annotations

from core.tests.base import MaintainerTestCase

from maintainers.users import services
from maintainers.users.models import User

from .factories import make_role, make_user


class ListUsersTests(MaintainerTestCase):
    def setUp(self):
        make_user("ana@example.com", name="Ana Torres")
        make_user("beto@example.com", name="Beto Díaz")
        make_user("carla@example.com", name="Carla Ruiz")

    def test_sin_search_devuelve_todos(self):
        data = services.list_users(search="", page=1, per_page=20)
        self.assertEqual(data["total"], 3)
        self.assertEqual(len(data["users"]), 3)
        self.assertEqual(data["total_pages"], 1)
        self.assertFalse(data["has_next"])
        self.assertFalse(data["has_prev"])

    def test_search_por_email(self):
        data = services.list_users(search="beto@", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["users"][0].email, "beto@example.com")

    def test_search_por_nombre(self):
        data = services.list_users(search="Carla", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["users"][0].email, "carla@example.com")

    def test_search_sin_match(self):
        data = services.list_users(search="zzz-no-existe", page=1, per_page=20)
        self.assertEqual(data["total"], 0)
        self.assertEqual(data["users"], [])

    def test_paginacion_parte_la_lista(self):
        data_p1 = services.list_users(search="", page=1, per_page=2)
        self.assertEqual(data_p1["total"], 3)
        self.assertEqual(len(data_p1["users"]), 2)
        self.assertEqual(data_p1["total_pages"], 2)
        self.assertTrue(data_p1["has_next"])
        self.assertFalse(data_p1["has_prev"])

        data_p2 = services.list_users(search="", page=2, per_page=2)
        self.assertEqual(len(data_p2["users"]), 1)
        self.assertFalse(data_p2["has_next"])
        self.assertTrue(data_p2["has_prev"])


class CreateUserTests(MaintainerTestCase):
    def setUp(self):
        make_role("viewer")

    def test_crea_usuario_ok(self):
        user = services.create_user(
            email="nuevo@example.com", name="Nuevo", role_slug="viewer",
            status="active", hashed_password=b"hash",
        )
        self.assertIsNotNone(user.pk)
        self.assertTrue(User.objects.filter(email="nuevo@example.com").exists())

    def test_email_duplicado_falla(self):
        make_user("dup@example.com")
        with self.assertRaises(services.UserError):
            services.create_user(
                email="dup@example.com", name="X", role_slug="viewer",
                status="active", hashed_password=b"hash",
            )

    def test_rol_inexistente_falla(self):
        with self.assertRaises(services.UserError):
            services.create_user(
                email="x@example.com", name="X", role_slug="rol-fantasma",
                status="active", hashed_password=b"hash",
            )


class UpdateUserTests(MaintainerTestCase):
    def setUp(self):
        make_role("viewer")
        make_role("admin")

    def test_actualiza_campos(self):
        u = make_user("edit@example.com", name="Viejo", role="viewer")
        services.update_user(
            u, email="edit@example.com", name="Nuevo Nombre",
            role_slug="admin", status="suspended", hashed_password=None,
        )
        u.refresh_from_db()
        self.assertEqual(u.name, "Nuevo Nombre")
        self.assertEqual(u.role, "admin")
        self.assertEqual(u.status, "suspended")

    def test_password_none_no_pisa(self):
        u = make_user("keep@example.com")
        u.password_hash = b"original"
        u.save()
        services.update_user(
            u, email="keep@example.com", name="", role_slug="viewer",
            status="active", hashed_password=None,
        )
        u.refresh_from_db()
        self.assertEqual(bytes(u.password_hash), b"original")

    def test_email_choca_con_otro_falla(self):
        make_user("ocupado@example.com")
        u = make_user("mio@example.com")
        with self.assertRaises(services.UserError):
            services.update_user(
                u, email="ocupado@example.com", name="", role_slug="viewer",
                status="active", hashed_password=None,
            )


class DeleteUserTests(MaintainerTestCase):
    def test_soft_delete_no_borra_fila(self):
        u = make_user("borrar@example.com")
        services.delete_user(u)
        u.refresh_from_db()
        self.assertIsNotNone(u.deleted_at)
        self.assertEqual(u.status, "revoked")
        self.assertTrue(User.objects.filter(pk=u.pk).exists())


class ToggleStatusTests(MaintainerTestCase):
    def test_active_a_suspended(self):
        u = make_user("t1@example.com", status="active")
        self.assertEqual(services.toggle_user_status(u), "suspended")

    def test_suspended_a_active(self):
        u = make_user("t2@example.com", status="suspended")
        self.assertEqual(services.toggle_user_status(u), "active")

    def test_pending_a_active(self):
        u = make_user("t3@example.com", status="pending")
        self.assertEqual(services.toggle_user_status(u), "active")

    def test_revoked_a_active(self):
        u = make_user("t4@example.com", status="revoked")
        self.assertEqual(services.toggle_user_status(u), "active")

    def test_persiste_en_db(self):
        u = make_user("t5@example.com", status="active")
        services.toggle_user_status(u)
        u.refresh_from_db()
        self.assertEqual(u.status, "suspended")


class ListSignalTests(MaintainerTestCase):
    def test_signal_cuenta_usuarios(self):
        make_user("s1@example.com")
        make_user("s2@example.com")
        sig = services.get_list_signal()
        self.assertEqual(sig["count"], 2)
        self.assertTrue(sig["version"])  # hay max(updated_at)

    def test_signal_cambia_tras_modificacion(self):
        u = make_user("s3@example.com", status="active")
        before = services.get_list_signal()
        services.toggle_user_status(u)  # bump updated_at
        after = services.get_list_signal()
        self.assertNotEqual(before["version"], after["version"])

    def test_signal_cambia_tras_alta(self):
        before = services.get_list_signal()
        make_user("s4@example.com")
        after = services.get_list_signal()
        self.assertEqual(after["count"], before["count"] + 1)
