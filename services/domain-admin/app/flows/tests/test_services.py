"""Tests de la capa de servicio del mantenedor de Flows.

Corren contra Postgres real (managed=True en settings_test). Cada assert
verifica un efecto observable en la DB o el valor de retorno real, no un mock.
"""
from __future__ import annotations

import uuid

from django.test import TestCase

from flows import services
from flows.models import Flow

from .factories import DEFAULT_ORG, make_flow, make_flow_version


class ListFlowsTests(TestCase):
    def setUp(self):
        make_flow("Onboarding", slug="onboarding", description="alta de usuarios")
        make_flow("Billing", slug="billing", description="facturación mensual")
        make_flow("Cleanup", slug="cleanup")

    def test_sin_search_devuelve_todos(self):
        data = services.list_flows(search="", page=1, per_page=20)
        self.assertEqual(data["total"], 3)
        self.assertEqual(len(data["flows"]), 3)
        self.assertEqual(data["total_pages"], 1)
        self.assertFalse(data["has_next"])
        self.assertFalse(data["has_prev"])

    def test_search_por_nombre(self):
        data = services.list_flows(search="Onboarding", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["flows"][0].slug, "onboarding")

    def test_search_por_slug(self):
        data = services.list_flows(search="billing", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["flows"][0].name, "Billing")

    def test_search_por_descripcion(self):
        data = services.list_flows(search="facturación", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["flows"][0].slug, "billing")

    def test_search_sin_match(self):
        data = services.list_flows(search="zzz-no-existe", page=1, per_page=20)
        self.assertEqual(data["total"], 0)
        self.assertEqual(data["flows"], [])

    def test_excluye_soft_deleted(self):
        make_flow("Borrado", slug="borrado", deleted=True)
        data = services.list_flows(search="", page=1, per_page=20)
        self.assertEqual(data["total"], 3)  # el borrado no cuenta

    def test_paginacion_parte_la_lista(self):
        data_p1 = services.list_flows(search="", page=1, per_page=2)
        self.assertEqual(data_p1["total"], 3)
        self.assertEqual(len(data_p1["flows"]), 2)
        self.assertEqual(data_p1["total_pages"], 2)
        self.assertTrue(data_p1["has_next"])
        self.assertFalse(data_p1["has_prev"])

        data_p2 = services.list_flows(search="", page=2, per_page=2)
        self.assertEqual(len(data_p2["flows"]), 1)
        self.assertFalse(data_p2["has_next"])
        self.assertTrue(data_p2["has_prev"])


class GetFlowTests(TestCase):
    def test_get_existente(self):
        f = make_flow("Existe", slug="existe")
        got = services.get_flow(f.pk)
        self.assertEqual(got.pk, f.pk)

    def test_get_inexistente_lanza_error(self):
        with self.assertRaises(services.FlowError):
            services.get_flow(uuid.uuid4())


class CreateFlowTests(TestCase):
    def test_crea_flow_ok(self):
        flow = services.create_flow(
            organization_id=DEFAULT_ORG, name="Nuevo", slug="nuevo",
            spec={"steps": [{"id": "a"}]}, is_active=True,
        )
        self.assertIsNotNone(flow.pk)
        self.assertTrue(Flow.objects.filter(slug="nuevo").exists())
        flow.refresh_from_db()
        self.assertEqual(flow.spec, {"steps": [{"id": "a"}]})

    def test_slug_duplicado_en_misma_org_falla(self):
        make_flow("Existente", slug="dup")
        with self.assertRaises(services.FlowError):
            services.create_flow(
                organization_id=DEFAULT_ORG, name="Otro", slug="dup", spec={},
            )

    def test_mismo_slug_otra_org_ok(self):
        make_flow("Existente", slug="shared")
        other_org = uuid.uuid4()
        flow = services.create_flow(
            organization_id=other_org, name="Otro", slug="shared", spec={},
        )
        self.assertIsNotNone(flow.pk)
        self.assertEqual(Flow.objects.filter(slug="shared").count(), 2)


class UpdateFlowTests(TestCase):
    def test_actualiza_campos(self):
        f = make_flow("Viejo", slug="viejo")
        services.update_flow(
            f, name="Nuevo Nombre", slug="viejo", description="cambiada",
            spec={"steps": [{"id": "b"}]}, is_active=False,
            deterministic_replay=True,
        )
        f.refresh_from_db()
        self.assertEqual(f.name, "Nuevo Nombre")
        self.assertEqual(f.description, "cambiada")
        self.assertFalse(f.is_active)
        self.assertTrue(f.deterministic_replay)
        self.assertEqual(f.spec, {"steps": [{"id": "b"}]})

    def test_cambia_slug_ok(self):
        f = make_flow("Flow", slug="antiguo")
        services.update_flow(f, name="Flow", slug="moderno", spec={})
        f.refresh_from_db()
        self.assertEqual(f.slug, "moderno")

    def test_slug_choca_con_otro_en_misma_org_falla(self):
        make_flow("Ocupado", slug="ocupado")
        f = make_flow("Mio", slug="mio")
        with self.assertRaises(services.FlowError):
            services.update_flow(f, name="Mio", slug="ocupado", spec={})

    def test_mantener_su_propio_slug_no_choca(self):
        f = make_flow("Flow", slug="estable")
        services.update_flow(f, name="Flow Editado", slug="estable", spec={})
        f.refresh_from_db()
        self.assertEqual(f.name, "Flow Editado")


class DeleteFlowTests(TestCase):
    def test_soft_delete_no_borra_fila(self):
        f = make_flow("Borrar", slug="borrar", is_active=True)
        services.delete_flow(f)
        f.refresh_from_db()
        self.assertIsNotNone(f.deleted_at)
        self.assertFalse(f.is_active)
        self.assertTrue(Flow.objects.filter(pk=f.pk).exists())


class ToggleStatusTests(TestCase):
    def test_active_a_inactive(self):
        f = make_flow("T1", slug="t1", is_active=True)
        self.assertFalse(services.toggle_flow_status(f))

    def test_inactive_a_active(self):
        f = make_flow("T2", slug="t2", is_active=False)
        self.assertTrue(services.toggle_flow_status(f))

    def test_persiste_en_db(self):
        f = make_flow("T3", slug="t3", is_active=True)
        services.toggle_flow_status(f)
        f.refresh_from_db()
        self.assertFalse(f.is_active)


class FlowVersionsTests(TestCase):
    def test_lista_versiones_ordenadas_desc(self):
        f = make_flow("Versionado", slug="versionado")
        make_flow_version(f, version=1)
        make_flow_version(f, version=2)
        make_flow_version(f, version=3)
        versions = services.get_flow_versions(f)
        self.assertEqual([v.version for v in versions], [3, 2, 1])

    def test_versiones_solo_del_flow_pedido(self):
        f1 = make_flow("F1", slug="f1")
        f2 = make_flow("F2", slug="f2")
        make_flow_version(f1, version=1)
        make_flow_version(f2, version=1)
        make_flow_version(f2, version=2)
        self.assertEqual(len(services.get_flow_versions(f1)), 1)
        self.assertEqual(len(services.get_flow_versions(f2)), 2)

    def test_flow_sin_versiones(self):
        f = make_flow("Sin", slug="sin")
        self.assertEqual(services.get_flow_versions(f), [])


class ListSignalTests(TestCase):
    def test_signal_cuenta_flows(self):
        make_flow("S1", slug="s1")
        make_flow("S2", slug="s2")
        sig = services.get_list_signal()
        self.assertEqual(sig["count"], 2)
        self.assertTrue(sig["version"])

    def test_signal_cambia_tras_modificacion(self):
        f = make_flow("S3", slug="s3", is_active=True)
        before = services.get_list_signal()
        services.toggle_flow_status(f)
        after = services.get_list_signal()
        self.assertNotEqual(before["version"], after["version"])

    def test_signal_cambia_tras_alta(self):
        before = services.get_list_signal()
        make_flow("S4", slug="s4")
        after = services.get_list_signal()
        self.assertEqual(after["count"], before["count"] + 1)


class StatsTests(TestCase):
    def test_stats_cuenta_por_estado(self):
        make_flow("A", slug="a", is_active=True)
        make_flow("B", slug="b", is_active=True)
        make_flow("C", slug="c", is_active=False)
        make_flow("D", slug="d", deleted=True)
        stats = services.get_stats()
        self.assertEqual(stats["total"], 3)  # el borrado no cuenta
        self.assertEqual(stats["active"], 2)
        self.assertEqual(stats["inactive"], 1)
