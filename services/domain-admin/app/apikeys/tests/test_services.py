"""Tests de la capa service del mantenedor de API Keys (DB real)."""
from __future__ import annotations

import hashlib

from django.test import TestCase
from django.utils import timezone

from apikeys import services
from apikeys.models import ApiKey

from .factories import make_api_key, make_user


class ListApiKeysTests(TestCase):
    def test_paginacion_y_total(self):
        owner = make_user("p@example.com")
        for i in range(25):
            make_api_key(f"key-{i}", user=owner, key_prefix=f"sk_p{i:03d}")
        data = services.list_api_keys(page=1, per_page=20)
        self.assertEqual(data["total"], 25)
        self.assertEqual(len(data["api_keys"]), 20)
        self.assertEqual(data["total_pages"], 2)
        self.assertTrue(data["has_next"])
        self.assertFalse(data["has_prev"])

    def test_busqueda_por_nombre(self):
        make_api_key("Produccion API", key_prefix="sk_prod0001")
        make_api_key("Staging API", key_prefix="sk_stag0001")
        data = services.list_api_keys(search="Produccion")
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["api_keys"][0].name, "Produccion API")

    def test_busqueda_por_prefijo(self):
        make_api_key("Una", key_prefix="sk_alpha0001")
        make_api_key("Otra", key_prefix="sk_beta00001")
        data = services.list_api_keys(search="alpha")
        self.assertEqual(data["total"], 1)


class CreateApiKeyTests(TestCase):
    def test_crea_y_devuelve_secreto(self):
        owner = make_user("c@example.com")
        api_key, secret = services.create_api_key(user=owner, name="Nueva")
        self.assertTrue(ApiKey.objects.filter(pk=api_key.pk).exists())
        self.assertTrue(secret.startswith("sk_"))
        # El hash persistido corresponde al secreto devuelto.
        self.assertEqual(api_key.key_hash, hashlib.sha256(secret.encode()).digest())
        # El prefijo persistido es el visible del secreto.
        self.assertEqual(api_key.key_prefix, secret[:20])

    def test_nombre_vacio_falla(self):
        owner = make_user("c2@example.com")
        with self.assertRaises(services.ApiKeyError):
            services.create_api_key(user=owner, name="   ")

    def test_nombre_duplicado_falla(self):
        owner = make_user("c3@example.com")
        services.create_api_key(user=owner, name="Dup")
        with self.assertRaises(services.ApiKeyError):
            services.create_api_key(user=owner, name="Dup")


class UpdateApiKeyTests(TestCase):
    def test_actualiza_nombre_y_expiracion(self):
        ak = make_api_key("Vieja")
        nuevo_exp = timezone.now() + timezone.timedelta(days=30)
        out = services.update_api_key(ak, name="Renombrada", expires_at=nuevo_exp)
        out.refresh_from_db()
        self.assertEqual(out.name, "Renombrada")
        self.assertIsNotNone(out.expires_at)

    def test_no_regenera_secreto(self):
        ak = make_api_key("Estable", key_prefix="sk_fixed0001")
        hash_antes = bytes(ak.key_hash)
        services.update_api_key(ak, name="Estable v2")
        ak.refresh_from_db()
        self.assertEqual(bytes(ak.key_hash), hash_antes)
        self.assertEqual(ak.key_prefix, "sk_fixed0001")


class DeleteApiKeyTests(TestCase):
    def test_soft_delete_marca_revoked(self):
        ak = make_api_key("Borrable", status="active")
        services.delete_api_key(ak)
        ak.refresh_from_db()
        self.assertEqual(ak.status, "revoked")
        self.assertIsNotNone(ak.revoked_at)
        # No se borró físicamente.
        self.assertTrue(ApiKey.objects.filter(pk=ak.pk).exists())


class ToggleApiKeyTests(TestCase):
    def test_toggle_revoca_y_reactiva(self):
        ak = make_api_key("Toggle", status="active")
        nuevo = services.toggle_api_key_status(ak)
        self.assertEqual(nuevo, "revoked")
        ak.refresh_from_db()
        self.assertIsNotNone(ak.revoked_at)

        nuevo2 = services.toggle_api_key_status(ak)
        self.assertEqual(nuevo2, "active")
        ak.refresh_from_db()
        self.assertIsNone(ak.revoked_at)


class SignalAndStatsTests(TestCase):
    def test_signal_cambia_al_crear(self):
        owner = make_user("s@example.com")
        sig0 = services.get_list_signal()
        self.assertEqual(sig0["count"], 0)
        services.create_api_key(user=owner, name="Sig")
        sig1 = services.get_list_signal()
        self.assertEqual(sig1["count"], 1)
        self.assertNotEqual(sig0["version"], sig1["version"])

    def test_stats(self):
        make_api_key("Activa", status="active")
        make_api_key("Revocada", revoked=True)
        stats = services.get_stats()
        self.assertEqual(stats["total"], 2)
        self.assertEqual(stats["active"], 1)
        self.assertEqual(stats["revoked"], 1)
