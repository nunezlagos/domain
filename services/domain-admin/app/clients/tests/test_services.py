"""Tests de la capa de servicio del mantenedor de Clientes.

Corren contra Postgres real (managed=True en settings_test). Cada assert
verifica un efecto observable en la DB o el valor de retorno real, no un mock.
"""
from __future__ import annotations

import uuid

from django.test import TestCase

from clients import services
from clients.models import Client

from .factories import DEFAULT_ORG, make_client


class ListClientsTests(TestCase):
    def setUp(self):
        make_client("Acme Corp", slug="acme", tax_id="76.111.111-1",
                    contact_email="ops@acme.com")
        make_client("Beta SpA", slug="beta", contact_email="hola@beta.com")
        make_client("Gamma Ltda", slug="gamma")

    def test_sin_search_devuelve_todos(self):
        data = services.list_clients(search="", page=1, per_page=20)
        self.assertEqual(data["total"], 3)
        self.assertEqual(len(data["clients"]), 3)
        self.assertEqual(data["total_pages"], 1)
        self.assertFalse(data["has_next"])
        self.assertFalse(data["has_prev"])

    def test_search_por_nombre(self):
        data = services.list_clients(search="Acme", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["clients"][0].slug, "acme")

    def test_search_por_slug(self):
        data = services.list_clients(search="beta", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["clients"][0].name, "Beta SpA")

    def test_search_por_email(self):
        data = services.list_clients(search="acme.com", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["clients"][0].slug, "acme")

    def test_search_sin_match(self):
        data = services.list_clients(search="zzz-no-existe", page=1, per_page=20)
        self.assertEqual(data["total"], 0)
        self.assertEqual(data["clients"], [])

    def test_excluye_soft_deleted(self):
        make_client("Borrado", slug="borrado", deleted=True)
        data = services.list_clients(search="", page=1, per_page=20)
        self.assertEqual(data["total"], 3)  # sigue siendo 3, el borrado no cuenta

    def test_paginacion_parte_la_lista(self):
        data_p1 = services.list_clients(search="", page=1, per_page=2)
        self.assertEqual(data_p1["total"], 3)
        self.assertEqual(len(data_p1["clients"]), 2)
        self.assertEqual(data_p1["total_pages"], 2)
        self.assertTrue(data_p1["has_next"])
        self.assertFalse(data_p1["has_prev"])

        data_p2 = services.list_clients(search="", page=2, per_page=2)
        self.assertEqual(len(data_p2["clients"]), 1)
        self.assertFalse(data_p2["has_next"])
        self.assertTrue(data_p2["has_prev"])


class CreateClientTests(TestCase):
    def test_crea_cliente_ok(self):
        client = services.create_client(
            organization_id=DEFAULT_ORG, name="Nuevo", slug="nuevo",
            contact_email="x@nuevo.com", status="active",
        )
        self.assertIsNotNone(client.pk)
        self.assertTrue(Client.objects.filter(slug="nuevo").exists())

    def test_slug_duplicado_en_misma_org_falla(self):
        make_client("Existente", slug="dup")
        with self.assertRaises(services.ClientError):
            services.create_client(
                organization_id=DEFAULT_ORG, name="Otro", slug="dup",
            )

    def test_mismo_slug_otra_org_ok(self):
        make_client("Existente", slug="shared")
        other_org = uuid.uuid4()
        client = services.create_client(
            organization_id=other_org, name="Otro", slug="shared",
        )
        self.assertIsNotNone(client.pk)
        self.assertEqual(Client.objects.filter(slug="shared").count(), 2)


class UpdateClientTests(TestCase):
    def test_actualiza_campos(self):
        c = make_client("Viejo", slug="viejo")
        services.update_client(
            c, name="Nuevo Nombre", slug="viejo", contact_email="nuevo@x.com",
            status="inactive",
        )
        c.refresh_from_db()
        self.assertEqual(c.name, "Nuevo Nombre")
        self.assertEqual(c.contact_email, "nuevo@x.com")
        self.assertEqual(c.status, "inactive")

    def test_cambia_slug_ok(self):
        c = make_client("Cliente", slug="antiguo")
        services.update_client(c, name="Cliente", slug="moderno", status="active")
        c.refresh_from_db()
        self.assertEqual(c.slug, "moderno")

    def test_slug_choca_con_otro_en_misma_org_falla(self):
        make_client("Ocupado", slug="ocupado")
        c = make_client("Mio", slug="mio")
        with self.assertRaises(services.ClientError):
            services.update_client(c, name="Mio", slug="ocupado", status="active")

    def test_mantener_su_propio_slug_no_choca(self):
        c = make_client("Cliente", slug="estable")
        services.update_client(c, name="Cliente Editado", slug="estable", status="active")
        c.refresh_from_db()
        self.assertEqual(c.name, "Cliente Editado")


class DeleteClientTests(TestCase):
    def test_soft_delete_no_borra_fila(self):
        c = make_client("Borrar", slug="borrar")
        services.delete_client(c)
        c.refresh_from_db()
        self.assertIsNotNone(c.deleted_at)
        self.assertEqual(c.status, "archived")
        self.assertTrue(Client.objects.filter(pk=c.pk).exists())


class ToggleStatusTests(TestCase):
    def test_active_a_inactive(self):
        c = make_client("T1", slug="t1", status="active")
        self.assertEqual(services.toggle_client_status(c), "inactive")

    def test_inactive_a_active(self):
        c = make_client("T2", slug="t2", status="inactive")
        self.assertEqual(services.toggle_client_status(c), "active")

    def test_archived_a_active(self):
        c = make_client("T3", slug="t3", status="archived")
        self.assertEqual(services.toggle_client_status(c), "active")

    def test_persiste_en_db(self):
        c = make_client("T4", slug="t4", status="active")
        services.toggle_client_status(c)
        c.refresh_from_db()
        self.assertEqual(c.status, "inactive")


class ListSignalTests(TestCase):
    def test_signal_cuenta_clientes(self):
        make_client("S1", slug="s1")
        make_client("S2", slug="s2")
        sig = services.get_list_signal()
        self.assertEqual(sig["count"], 2)
        self.assertTrue(sig["version"])

    def test_signal_cambia_tras_modificacion(self):
        c = make_client("S3", slug="s3", status="active")
        before = services.get_list_signal()
        services.toggle_client_status(c)
        after = services.get_list_signal()
        self.assertNotEqual(before["version"], after["version"])

    def test_signal_cambia_tras_alta(self):
        before = services.get_list_signal()
        make_client("S4", slug="s4")
        after = services.get_list_signal()
        self.assertEqual(after["count"], before["count"] + 1)


class StatsTests(TestCase):
    def test_stats_cuenta_por_estado(self):
        make_client("A", slug="a", status="active")
        make_client("B", slug="b", status="active")
        make_client("C", slug="c", status="inactive")
        make_client("D", slug="d", deleted=True)
        stats = services.get_stats()
        self.assertEqual(stats["total"], 3)  # el borrado no cuenta
        self.assertEqual(stats["active"], 2)
        self.assertEqual(stats["inactive"], 1)
