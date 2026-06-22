"""Tests de la capa de servicio del mantenedor de Plantillas de Agentes.

Corren contra Postgres real (managed=True via core.tests.runner). Cada assert
verifica un efecto observable en la DB o el valor de retorno real, no un mock.
"""
from __future__ import annotations

from core.tests.base import MaintainerTestCase

from maintainers.agenttemplates import services
from maintainers.agenttemplates.models import AgentTemplate

from .factories import make_agent_template


class ListAgentTemplatesTests(MaintainerTestCase):
    def setUp(self):
        make_agent_template("Orquestador", slug="orquestador",
                             role="orchestrator")
        make_agent_template("Trabajador Fase", slug="trabajador-fase",
                             role="phase-worker")
        make_agent_template("Generador", slug="generador")

    def test_sin_search_devuelve_todos(self):
        data = services.list_agenttemplates(search="", page=1, per_page=20)
        self.assertEqual(data["total"], 3)
        self.assertEqual(len(data["agenttemplates"]), 3)
        self.assertEqual(data["total_pages"], 1)
        self.assertFalse(data["has_next"])
        self.assertFalse(data["has_prev"])

    def test_search_por_nombre(self):
        data = services.list_agenttemplates(search="Orquestador", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["agenttemplates"][0].slug, "orquestador")

    def test_search_por_slug(self):
        data = services.list_agenttemplates(search="generador", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["agenttemplates"][0].name, "Generador")

    def test_search_por_rol(self):
        data = services.list_agenttemplates(search="orchestrator", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["agenttemplates"][0].slug, "orquestador")

    def test_search_sin_match(self):
        data = services.list_agenttemplates(search="zzz-no-existe", page=1, per_page=20)
        self.assertEqual(data["total"], 0)
        self.assertEqual(data["agenttemplates"], [])

    def test_orden_por_nombre(self):
        data = services.list_agenttemplates(search="", page=1, per_page=20)
        nombres = [t.name for t in data["agenttemplates"]]
        self.assertEqual(nombres, sorted(nombres))

    def test_paginacion_parte_la_lista(self):
        data_p1 = services.list_agenttemplates(search="", page=1, per_page=2)
        self.assertEqual(data_p1["total"], 3)
        self.assertEqual(len(data_p1["agenttemplates"]), 2)
        self.assertEqual(data_p1["total_pages"], 2)
        self.assertTrue(data_p1["has_next"])
        self.assertFalse(data_p1["has_prev"])

        data_p2 = services.list_agenttemplates(search="", page=2, per_page=2)
        self.assertEqual(len(data_p2["agenttemplates"]), 1)
        self.assertFalse(data_p2["has_next"])
        self.assertTrue(data_p2["has_prev"])


class CreateAgentTemplateTests(MaintainerTestCase):
    def test_crea_template_ok(self):
        t = services.create_agenttemplate(
            slug="nueva", name="Nueva", system_prompt="Sos util.",
            capabilities=["research"],
        )
        self.assertIsNotNone(t.pk)
        self.assertTrue(AgentTemplate.objects.filter(slug="nueva").exists())
        self.assertEqual(t.capabilities, ["research"])

    def test_slug_duplicado_falla(self):
        make_agent_template("Existente", slug="dup")
        with self.assertRaises(services.AgentTemplateError):
            services.create_agenttemplate(
                slug="dup", name="Otra", system_prompt="x",
            )


class UpdateAgentTemplateTests(MaintainerTestCase):
    def test_actualiza_campos(self):
        t = make_agent_template("Vieja", slug="vieja")
        services.update_agenttemplate(
            t, slug="vieja", name="Nuevo Nombre", system_prompt="nuevo prompt",
            capabilities=["a", "b"], max_tokens=8192, role="orchestrator",
        )
        t.refresh_from_db()
        self.assertEqual(t.name, "Nuevo Nombre")
        self.assertEqual(t.system_prompt, "nuevo prompt")
        self.assertEqual(t.max_tokens, 8192)
        self.assertEqual(t.capabilities, ["a", "b"])
        self.assertEqual(t.role, "orchestrator")

    def test_cambia_slug_ok(self):
        t = make_agent_template("Plantilla", slug="antiguo")
        services.update_agenttemplate(t, slug="moderno", name="Plantilla",
                                      system_prompt="x")
        t.refresh_from_db()
        self.assertEqual(t.slug, "moderno")

    def test_slug_choca_con_otro_falla(self):
        make_agent_template("Ocupado", slug="ocupado")
        t = make_agent_template("Mio", slug="mio")
        with self.assertRaises(services.AgentTemplateError):
            services.update_agenttemplate(t, slug="ocupado", name="Mio",
                                          system_prompt="x")

    def test_mantener_su_propio_slug_no_choca(self):
        t = make_agent_template("Plantilla", slug="estable")
        services.update_agenttemplate(t, slug="estable", name="Editada",
                                      system_prompt="x")
        t.refresh_from_db()
        self.assertEqual(t.name, "Editada")


class DeleteAgentTemplateTests(MaintainerTestCase):
    def test_hard_delete_borra_fila(self):
        t = make_agent_template("Borrar", slug="borrar")
        pk = t.pk
        services.delete_agenttemplate(t)
        self.assertFalse(AgentTemplate.objects.filter(pk=pk).exists())


class ListSignalTests(MaintainerTestCase):
    def test_signal_cuenta_templates(self):
        make_agent_template("S1", slug="s1")
        make_agent_template("S2", slug="s2")
        sig = services.get_list_signal()
        self.assertEqual(sig["count"], 2)
        self.assertTrue(sig["version"])

    def test_signal_cambia_tras_modificacion(self):
        t = make_agent_template("S3", slug="s3")
        before = services.get_list_signal()
        services.update_agenttemplate(t, slug="s3", name="S3 Editada",
                                      system_prompt="x")
        after = services.get_list_signal()
        self.assertNotEqual(before["version"], after["version"])

    def test_signal_cambia_tras_alta(self):
        before = services.get_list_signal()
        make_agent_template("S4", slug="s4")
        after = services.get_list_signal()
        self.assertEqual(after["count"], before["count"] + 1)
