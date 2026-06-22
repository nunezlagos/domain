"""Tests de la capa de servicio del mantenedor de Skills.

Corren contra Postgres real (managed=True via core.tests.runner). Cada assert
verifica un efecto observable en la DB o el valor de retorno real, no un mock.
"""
from __future__ import annotations

from core.tests.base import MaintainerTestCase

from maintainers.skills import services
from maintainers.skills.models import Skill

from .factories import DEFAULT_PROJECT, make_skill, make_skill_version


class ListSkillsTests(MaintainerTestCase):
    def setUp(self):
        make_skill("Resumir Ticket", slug="resumir-ticket",
                   description="Resume un ticket de soporte")
        make_skill("Clasificar Intencion", slug="clasificar-intencion",
                   description="Detecta la intencion del usuario")
        make_skill("Generar Respuesta", slug="generar-respuesta")

    def test_sin_search_devuelve_todos(self):
        data = services.list_skills(search="", page=1, per_page=20)
        self.assertEqual(data["total"], 3)
        self.assertEqual(len(data["skills"]), 3)
        self.assertEqual(data["total_pages"], 1)
        self.assertFalse(data["has_next"])
        self.assertFalse(data["has_prev"])

    def test_search_por_nombre(self):
        data = services.list_skills(search="Resumir", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["skills"][0].slug, "resumir-ticket")

    def test_search_por_slug(self):
        data = services.list_skills(search="clasificar-intencion", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["skills"][0].name, "Clasificar Intencion")

    def test_search_por_descripcion(self):
        data = services.list_skills(search="soporte", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["skills"][0].slug, "resumir-ticket")

    def test_search_sin_match(self):
        data = services.list_skills(search="zzz-no-existe", page=1, per_page=20)
        self.assertEqual(data["total"], 0)
        self.assertEqual(data["skills"], [])

    def test_excluye_soft_deleted(self):
        make_skill("Borrada", slug="borrada", deleted=True)
        data = services.list_skills(search="", page=1, per_page=20)
        self.assertEqual(data["total"], 3)  # el borrado no cuenta

    def test_paginacion_parte_la_lista(self):
        data_p1 = services.list_skills(search="", page=1, per_page=2)
        self.assertEqual(data_p1["total"], 3)
        self.assertEqual(len(data_p1["skills"]), 2)
        self.assertEqual(data_p1["total_pages"], 2)
        self.assertTrue(data_p1["has_next"])
        self.assertFalse(data_p1["has_prev"])

        data_p2 = services.list_skills(search="", page=2, per_page=2)
        self.assertEqual(len(data_p2["skills"]), 1)
        self.assertFalse(data_p2["has_next"])
        self.assertTrue(data_p2["has_prev"])


class CreateSkillTests(MaintainerTestCase):
    def test_crea_skill_ok(self):
        skill = services.create_skill(
            slug="nueva", name="Nueva", skill_type="prompt",
            tags=["soporte"],
        )
        self.assertIsNotNone(skill.pk)
        self.assertTrue(Skill.objects.filter(slug="nueva").exists())
        self.assertEqual(skill.tags, ["soporte"])

    def test_slug_duplicado_en_mismo_scope_global_falla(self):
        make_skill("Existente", slug="dup")  # global (project_id None)
        with self.assertRaises(services.SkillError):
            services.create_skill(slug="dup", name="Otra")

    def test_mismo_slug_otro_scope_ok(self):
        make_skill("Global", slug="shared")  # project_id None
        skill = services.create_skill(
            slug="shared", name="De Proyecto", project_id=DEFAULT_PROJECT,
        )
        self.assertIsNotNone(skill.pk)
        self.assertEqual(Skill.objects.filter(slug="shared").count(), 2)

    def test_slug_de_skill_borrada_se_puede_reusar(self):
        make_skill("Vieja", slug="reusable", deleted=True)
        skill = services.create_skill(slug="reusable", name="Nueva")
        self.assertIsNotNone(skill.pk)


class UpdateSkillTests(MaintainerTestCase):
    def test_actualiza_campos(self):
        s = make_skill("Vieja", slug="vieja")
        services.update_skill(
            s, slug="vieja", name="Nuevo Nombre", skill_type="prompt",
            description="desc", timeout_seconds=60, tags=["a", "b"],
        )
        s.refresh_from_db()
        self.assertEqual(s.name, "Nuevo Nombre")
        self.assertEqual(s.description, "desc")
        self.assertEqual(s.timeout_seconds, 60)
        self.assertEqual(s.tags, ["a", "b"])

    def test_cambia_slug_ok(self):
        s = make_skill("Skill", slug="antiguo")
        services.update_skill(s, slug="moderno", name="Skill")
        s.refresh_from_db()
        self.assertEqual(s.slug, "moderno")

    def test_slug_choca_con_otro_en_mismo_scope_falla(self):
        make_skill("Ocupado", slug="ocupado")
        s = make_skill("Mio", slug="mio")
        with self.assertRaises(services.SkillError):
            services.update_skill(s, slug="ocupado", name="Mio")

    def test_mantener_su_propio_slug_no_choca(self):
        s = make_skill("Skill", slug="estable")
        services.update_skill(s, slug="estable", name="Skill Editada")
        s.refresh_from_db()
        self.assertEqual(s.name, "Skill Editada")


class DeleteSkillTests(MaintainerTestCase):
    def test_soft_delete_no_borra_fila(self):
        s = make_skill("Borrar", slug="borrar")
        services.delete_skill(s)
        s.refresh_from_db()
        self.assertIsNotNone(s.deleted_at)
        self.assertTrue(Skill.objects.filter(pk=s.pk).exists())


class SkillVersionsTests(MaintainerTestCase):
    def test_getter_devuelve_versiones_ordenadas_desc(self):
        s = make_skill("Versionada", slug="versionada")
        make_skill_version(s, version=1, changelog="inicial")
        make_skill_version(s, version=2, changelog="mejora")
        versions = services.get_skill_versions(s)
        self.assertEqual([v.version for v in versions], [2, 1])

    def test_solo_versiones_de_la_skill(self):
        s1 = make_skill("S1", slug="s1")
        s2 = make_skill("S2", slug="s2")
        make_skill_version(s1, version=1)
        make_skill_version(s2, version=1)
        make_skill_version(s2, version=2)
        self.assertEqual(len(services.get_skill_versions(s1)), 1)
        self.assertEqual(len(services.get_skill_versions(s2)), 2)


class ListSignalTests(MaintainerTestCase):
    def test_signal_cuenta_skills(self):
        make_skill("S1", slug="s1")
        make_skill("S2", slug="s2")
        sig = services.get_list_signal()
        self.assertEqual(sig["count"], 2)
        self.assertTrue(sig["version"])

    def test_signal_cambia_tras_modificacion(self):
        s = make_skill("S3", slug="s3")
        before = services.get_list_signal()
        services.update_skill(s, slug="s3", name="S3 Editada")
        after = services.get_list_signal()
        self.assertNotEqual(before["version"], after["version"])

    def test_signal_cambia_tras_alta(self):
        before = services.get_list_signal()
        make_skill("S4", slug="s4")
        after = services.get_list_signal()
        self.assertEqual(after["count"], before["count"] + 1)


class StatsTests(MaintainerTestCase):
    def test_stats_cuenta(self):
        make_skill("A", slug="a", skill_type="prompt")
        make_skill("B", slug="b", skill_type="prompt")
        make_skill("C", slug="c", proposed=True)
        make_skill("D", slug="d", deleted=True)
        stats = services.get_stats()
        self.assertEqual(stats["total"], 3)  # el borrado no cuenta
        self.assertEqual(stats["prompt"], 3)
        self.assertEqual(stats["proposed"], 1)
