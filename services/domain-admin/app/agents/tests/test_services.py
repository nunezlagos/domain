"""Tests de la capa de servicio del mantenedor de Agentes.

Corren contra Postgres real (managed=True en settings_test). Cada assert
verifica un efecto observable en la DB o el valor de retorno real, no un mock.
"""
from __future__ import annotations

import uuid

from django.test import TestCase

from agents import services
from agents.models import Agent

from .factories import (
    make_agent,
    make_agent_template,
    make_agent_version,
)


class ListAgentsTests(TestCase):
    def setUp(self):
        make_agent("Soporte Bot", slug="soporte", provider="anthropic",
                   model="claude-haiku-4-5")
        make_agent("Ventas Bot", slug="ventas", provider="openai",
                   model="gpt-4o")
        make_agent("Research Bot", slug="research", provider="google",
                   model="gemini-1.5-pro")

    def test_sin_search_devuelve_todos(self):
        data = services.list_agents(search="", page=1, per_page=20)
        self.assertEqual(data["total"], 3)
        self.assertEqual(len(data["agents"]), 3)
        self.assertEqual(data["total_pages"], 1)
        self.assertFalse(data["has_next"])
        self.assertFalse(data["has_prev"])

    def test_search_por_nombre(self):
        data = services.list_agents(search="Soporte", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["agents"][0].slug, "soporte")

    def test_search_por_slug(self):
        data = services.list_agents(search="ventas", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["agents"][0].name, "Ventas Bot")

    def test_search_por_provider(self):
        data = services.list_agents(search="openai", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["agents"][0].slug, "ventas")

    def test_search_por_model(self):
        data = services.list_agents(search="gemini", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["agents"][0].slug, "research")

    def test_search_sin_match(self):
        data = services.list_agents(search="zzz-no-existe", page=1, per_page=20)
        self.assertEqual(data["total"], 0)
        self.assertEqual(data["agents"], [])

    def test_excluye_soft_deleted(self):
        make_agent("Borrado", slug="borrado", deleted=True)
        data = services.list_agents(search="", page=1, per_page=20)
        self.assertEqual(data["total"], 3)  # el borrado no cuenta

    def test_paginacion_parte_la_lista(self):
        data_p1 = services.list_agents(search="", page=1, per_page=2)
        self.assertEqual(data_p1["total"], 3)
        self.assertEqual(len(data_p1["agents"]), 2)
        self.assertEqual(data_p1["total_pages"], 2)
        self.assertTrue(data_p1["has_next"])
        self.assertFalse(data_p1["has_prev"])

        data_p2 = services.list_agents(search="", page=2, per_page=2)
        self.assertEqual(len(data_p2["agents"]), 1)
        self.assertFalse(data_p2["has_next"])
        self.assertTrue(data_p2["has_prev"])


class GetAgentTests(TestCase):
    def test_get_existente(self):
        a = make_agent("Uno", slug="uno")
        got = services.get_agent(a.pk)
        self.assertEqual(got.pk, a.pk)

    def test_get_inexistente_levanta_error(self):
        with self.assertRaises(services.AgentError):
            services.get_agent(uuid.uuid4())


class CreateAgentTests(TestCase):
    def test_crea_agente_ok(self):
        agent = services.create_agent(
            name="Nuevo", slug="nuevo",
            provider="anthropic", model="claude-haiku-4-5",
            skills_slugs=["search", "summarize"],
        )
        self.assertIsNotNone(agent.pk)
        self.assertTrue(Agent.objects.filter(slug="nuevo").exists())
        agent.refresh_from_db()
        self.assertEqual(agent.skills_slugs, ["search", "summarize"])

    def test_slug_duplicado_falla(self):
        make_agent("Existente", slug="dup")
        with self.assertRaises(services.AgentError):
            services.create_agent(
                name="Otro", slug="dup",
                provider="anthropic", model="claude-haiku-4-5",
            )


class UpdateAgentTests(TestCase):
    def test_actualiza_campos(self):
        a = make_agent("Viejo", slug="viejo", provider="anthropic")
        services.update_agent(
            a, name="Nuevo Nombre", slug="viejo", provider="openai",
            model="gpt-4o", skills_slugs=["x"], max_iterations=50,
        )
        a.refresh_from_db()
        self.assertEqual(a.name, "Nuevo Nombre")
        self.assertEqual(a.provider, "openai")
        self.assertEqual(a.model, "gpt-4o")
        self.assertEqual(a.skills_slugs, ["x"])
        self.assertEqual(a.max_iterations, 50)

    def test_cambia_slug_ok(self):
        a = make_agent("Agente", slug="antiguo")
        services.update_agent(a, name="Agente", slug="moderno",
                              provider="anthropic", model="claude-haiku-4-5")
        a.refresh_from_db()
        self.assertEqual(a.slug, "moderno")

    def test_slug_choca_con_otro_falla(self):
        make_agent("Ocupado", slug="ocupado")
        a = make_agent("Mio", slug="mio")
        with self.assertRaises(services.AgentError):
            services.update_agent(a, name="Mio", slug="ocupado",
                                  provider="anthropic", model="claude-haiku-4-5")

    def test_mantener_su_propio_slug_no_choca(self):
        a = make_agent("Agente", slug="estable")
        services.update_agent(a, name="Agente Editado", slug="estable",
                              provider="anthropic", model="claude-haiku-4-5")
        a.refresh_from_db()
        self.assertEqual(a.name, "Agente Editado")


class DeleteAgentTests(TestCase):
    def test_soft_delete_no_borra_fila(self):
        a = make_agent("Borrar", slug="borrar")
        services.delete_agent(a)
        a.refresh_from_db()
        self.assertIsNotNone(a.deleted_at)
        self.assertTrue(Agent.objects.filter(pk=a.pk).exists())


class AgentVersionsTests(TestCase):
    def test_devuelve_versiones_mas_reciente_primero(self):
        a = make_agent("Versionado", slug="versionado")
        make_agent_version(a, 1)
        make_agent_version(a, 2)
        make_agent_version(a, 3)
        versions = services.get_agent_versions(a)
        self.assertEqual([v.version for v in versions], [3, 2, 1])

    def test_solo_versiones_del_agent(self):
        a = make_agent("A", slug="a")
        b = make_agent("B", slug="b")
        make_agent_version(a, 1)
        make_agent_version(b, 1)
        make_agent_version(b, 2)
        self.assertEqual(len(services.get_agent_versions(a)), 1)
        self.assertEqual(len(services.get_agent_versions(b)), 2)

    def test_sin_versiones_lista_vacia(self):
        a = make_agent("Sin", slug="sin")
        self.assertEqual(services.get_agent_versions(a), [])


class AgentTemplatesTests(TestCase):
    def test_devuelve_templates_ordenados_por_nombre(self):
        make_agent_template("Zeta", slug="zeta")
        make_agent_template("Alfa", slug="alfa")
        templates = services.get_agent_templates()
        self.assertEqual([t.name for t in templates], ["Alfa", "Zeta"])

    def test_sin_templates_lista_vacia(self):
        self.assertEqual(services.get_agent_templates(), [])


class ListSignalTests(TestCase):
    def test_signal_cuenta_agentes(self):
        make_agent("S1", slug="s1")
        make_agent("S2", slug="s2")
        sig = services.get_list_signal()
        self.assertEqual(sig["count"], 2)
        self.assertTrue(sig["version"])

    def test_signal_cambia_tras_modificacion(self):
        a = make_agent("S3", slug="s3")
        before = services.get_list_signal()
        services.update_agent(a, name="S3 editado", slug="s3",
                              provider="anthropic", model="claude-haiku-4-5")
        after = services.get_list_signal()
        self.assertNotEqual(before["version"], after["version"])

    def test_signal_cambia_tras_alta(self):
        before = services.get_list_signal()
        make_agent("S4", slug="s4")
        after = services.get_list_signal()
        self.assertEqual(after["count"], before["count"] + 1)


class StatsTests(TestCase):
    def test_stats_cuenta_por_flag(self):
        make_agent("A", slug="a", seed_managed=True)
        make_agent("B", slug="b", is_user_modified=True)
        make_agent("C", slug="c")
        make_agent("D", slug="d", deleted=True)
        stats = services.get_stats()
        self.assertEqual(stats["total"], 3)  # el borrado no cuenta
        self.assertEqual(stats["seed_managed"], 1)
        self.assertEqual(stats["user_modified"], 1)
