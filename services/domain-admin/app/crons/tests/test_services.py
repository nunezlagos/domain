"""Tests de la capa de servicio del mantenedor de Crons.

Corren contra Postgres real (managed=True en settings_test). Cada assert
verifica un efecto observable en la DB o el valor de retorno real, no un mock.
"""
from __future__ import annotations

import uuid

from django.test import TestCase

from crons import services
from crons.models import Cron

from .factories import DEFAULT_ORG, DEFAULT_TARGET, make_cron


class ListCronsTests(TestCase):
    def setUp(self):
        make_cron("Daily Report", slug="daily-report",
                  cron_expression="0 9 * * *", target_type="flow")
        make_cron("Hourly Sync", slug="hourly-sync",
                  cron_expression="0 * * * *", target_type="agent")
        make_cron("Weekly Cleanup", slug="weekly-cleanup",
                  cron_expression="0 0 * * 0", target_type="skill")

    def test_sin_search_devuelve_todos(self):
        data = services.list_crons(search="", page=1, per_page=20)
        self.assertEqual(data["total"], 3)
        self.assertEqual(len(data["crons"]), 3)
        self.assertEqual(data["total_pages"], 1)
        self.assertFalse(data["has_next"])
        self.assertFalse(data["has_prev"])

    def test_search_por_nombre(self):
        data = services.list_crons(search="Daily", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["crons"][0].slug, "daily-report")

    def test_search_por_slug(self):
        data = services.list_crons(search="hourly-sync", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["crons"][0].name, "Hourly Sync")

    def test_search_por_expresion(self):
        data = services.list_crons(search="0 0 * * 0", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["crons"][0].slug, "weekly-cleanup")

    def test_search_por_target_type(self):
        data = services.list_crons(search="agent", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["crons"][0].slug, "hourly-sync")

    def test_search_sin_match(self):
        data = services.list_crons(search="zzz-no-existe", page=1, per_page=20)
        self.assertEqual(data["total"], 0)
        self.assertEqual(data["crons"], [])

    def test_excluye_soft_deleted(self):
        make_cron("Borrado", slug="borrado", deleted=True)
        data = services.list_crons(search="", page=1, per_page=20)
        self.assertEqual(data["total"], 3)  # el borrado no cuenta

    def test_paginacion_parte_la_lista(self):
        data_p1 = services.list_crons(search="", page=1, per_page=2)
        self.assertEqual(data_p1["total"], 3)
        self.assertEqual(len(data_p1["crons"]), 2)
        self.assertEqual(data_p1["total_pages"], 2)
        self.assertTrue(data_p1["has_next"])
        self.assertFalse(data_p1["has_prev"])

        data_p2 = services.list_crons(search="", page=2, per_page=2)
        self.assertEqual(len(data_p2["crons"]), 1)
        self.assertFalse(data_p2["has_next"])
        self.assertTrue(data_p2["has_prev"])


class CreateCronTests(TestCase):
    def test_crea_cron_ok(self):
        cron = services.create_cron(
            organization_id=DEFAULT_ORG, name="Nuevo", slug="nuevo",
            cron_expression="*/5 * * * *", target_type="flow",
            target_id=DEFAULT_TARGET, inputs={"k": "v"}, enabled=True,
        )
        self.assertIsNotNone(cron.pk)
        self.assertTrue(Cron.objects.filter(slug="nuevo").exists())
        cron.refresh_from_db()
        self.assertEqual(cron.inputs, {"k": "v"})
        self.assertTrue(cron.enabled)

    def test_slug_duplicado_en_misma_org_falla(self):
        make_cron("Existente", slug="dup")
        with self.assertRaises(services.CronError):
            services.create_cron(
                organization_id=DEFAULT_ORG, name="Otro", slug="dup",
                cron_expression="0 0 * * *", target_type="flow",
                target_id=DEFAULT_TARGET,
            )

    def test_mismo_slug_otra_org_ok(self):
        make_cron("Existente", slug="shared")
        other_org = uuid.uuid4()
        cron = services.create_cron(
            organization_id=other_org, name="Otro", slug="shared",
            cron_expression="0 0 * * *", target_type="agent",
            target_id=DEFAULT_TARGET,
        )
        self.assertIsNotNone(cron.pk)
        self.assertEqual(Cron.objects.filter(slug="shared").count(), 2)


class UpdateCronTests(TestCase):
    def test_actualiza_campos(self):
        c = make_cron("Viejo", slug="viejo")
        services.update_cron(
            c, name="Nuevo Nombre", slug="viejo",
            cron_expression="0 12 * * *", target_type="skill",
            target_id=DEFAULT_TARGET, inputs={"a": 1}, enabled=False,
        )
        c.refresh_from_db()
        self.assertEqual(c.name, "Nuevo Nombre")
        self.assertEqual(c.cron_expression, "0 12 * * *")
        self.assertEqual(c.target_type, "skill")
        self.assertEqual(c.inputs, {"a": 1})
        self.assertFalse(c.enabled)

    def test_cambia_slug_ok(self):
        c = make_cron("Cron", slug="antiguo")
        services.update_cron(
            c, name="Cron", slug="moderno",
            cron_expression="0 0 * * *", target_type="flow",
            target_id=DEFAULT_TARGET,
        )
        c.refresh_from_db()
        self.assertEqual(c.slug, "moderno")

    def test_slug_choca_con_otro_en_misma_org_falla(self):
        make_cron("Ocupado", slug="ocupado")
        c = make_cron("Mio", slug="mio")
        with self.assertRaises(services.CronError):
            services.update_cron(
                c, name="Mio", slug="ocupado",
                cron_expression="0 0 * * *", target_type="flow",
                target_id=DEFAULT_TARGET,
            )

    def test_mantener_su_propio_slug_no_choca(self):
        c = make_cron("Cron", slug="estable")
        services.update_cron(
            c, name="Cron Editado", slug="estable",
            cron_expression="0 0 * * *", target_type="flow",
            target_id=DEFAULT_TARGET,
        )
        c.refresh_from_db()
        self.assertEqual(c.name, "Cron Editado")


class DeleteCronTests(TestCase):
    def test_soft_delete_no_borra_fila(self):
        c = make_cron("Borrar", slug="borrar", enabled=True)
        services.delete_cron(c)
        c.refresh_from_db()
        self.assertIsNotNone(c.deleted_at)
        self.assertFalse(c.enabled)
        self.assertTrue(Cron.objects.filter(pk=c.pk).exists())


class ToggleEnabledTests(TestCase):
    def test_enabled_a_disabled(self):
        c = make_cron("T1", slug="t1", enabled=True)
        self.assertFalse(services.toggle_cron_enabled(c))

    def test_disabled_a_enabled(self):
        c = make_cron("T2", slug="t2", enabled=False)
        self.assertTrue(services.toggle_cron_enabled(c))

    def test_persiste_en_db(self):
        c = make_cron("T3", slug="t3", enabled=True)
        services.toggle_cron_enabled(c)
        c.refresh_from_db()
        self.assertFalse(c.enabled)


class ListSignalTests(TestCase):
    def test_signal_cuenta_crons(self):
        make_cron("S1", slug="s1")
        make_cron("S2", slug="s2")
        sig = services.get_list_signal()
        self.assertEqual(sig["count"], 2)
        self.assertTrue(sig["version"])

    def test_signal_cambia_tras_modificacion(self):
        c = make_cron("S3", slug="s3", enabled=True)
        before = services.get_list_signal()
        services.toggle_cron_enabled(c)
        after = services.get_list_signal()
        self.assertNotEqual(before["version"], after["version"])

    def test_signal_cambia_tras_alta(self):
        before = services.get_list_signal()
        make_cron("S4", slug="s4")
        after = services.get_list_signal()
        self.assertEqual(after["count"], before["count"] + 1)


class StatsTests(TestCase):
    def test_stats_cuenta_por_estado(self):
        make_cron("A", slug="a", enabled=True)
        make_cron("B", slug="b", enabled=True)
        make_cron("C", slug="c", enabled=False)
        make_cron("D", slug="d", deleted=True)
        stats = services.get_stats()
        self.assertEqual(stats["total"], 3)  # el borrado no cuenta
        self.assertEqual(stats["enabled"], 2)
        self.assertEqual(stats["disabled"], 1)
