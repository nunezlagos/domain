"""Tests de la capa de servicio del mantenedor de Prompts.

Corren contra Postgres real (managed=True en settings_test). Cada assert
verifica un efecto observable en la DB o el valor de retorno real, no un mock.
"""
from __future__ import annotations

import uuid

from django.test import TestCase

from prompts import services
from prompts.models import Prompt

from .factories import make_prompt


class ListPromptsTests(TestCase):
    def setUp(self):
        make_prompt("saludo", body="Hola, soy tu asistente.",
                    description="Mensaje de bienvenida")
        make_prompt("despedida", body="Gracias por tu visita.")
        make_prompt("escalamiento", body="Te derivo a un humano.")

    def test_sin_search_devuelve_todos(self):
        data = services.list_prompts(search="", page=1, per_page=20)
        self.assertEqual(data["total"], 3)
        self.assertEqual(len(data["prompts"]), 3)
        self.assertEqual(data["total_pages"], 1)
        self.assertFalse(data["has_next"])
        self.assertFalse(data["has_prev"])

    def test_search_por_slug(self):
        data = services.list_prompts(search="saludo", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["prompts"][0].slug, "saludo")

    def test_search_por_descripcion(self):
        data = services.list_prompts(search="bienvenida", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["prompts"][0].slug, "saludo")

    def test_search_por_body(self):
        data = services.list_prompts(search="humano", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["prompts"][0].slug, "escalamiento")

    def test_search_sin_match(self):
        data = services.list_prompts(search="zzz-no-existe", page=1, per_page=20)
        self.assertEqual(data["total"], 0)
        self.assertEqual(data["prompts"], [])

    def test_excluye_soft_deleted(self):
        make_prompt("borrado", deleted=True)
        data = services.list_prompts(search="", page=1, per_page=20)
        self.assertEqual(data["total"], 3)  # el borrado no cuenta

    def test_paginacion_parte_la_lista(self):
        data_p1 = services.list_prompts(search="", page=1, per_page=2)
        self.assertEqual(data_p1["total"], 3)
        self.assertEqual(len(data_p1["prompts"]), 2)
        self.assertEqual(data_p1["total_pages"], 2)
        self.assertTrue(data_p1["has_next"])
        self.assertFalse(data_p1["has_prev"])

        data_p2 = services.list_prompts(search="", page=2, per_page=2)
        self.assertEqual(len(data_p2["prompts"]), 1)
        self.assertFalse(data_p2["has_next"])
        self.assertTrue(data_p2["has_prev"])


class CreatePromptTests(TestCase):
    def test_crea_prompt_ok(self):
        prompt = services.create_prompt(
            slug="nuevo", body="Cuerpo.",
            version=1, description="desc", tags=["a", "b"],
        )
        self.assertIsNotNone(prompt.pk)
        self.assertTrue(Prompt.objects.filter(slug="nuevo").exists())
        prompt.refresh_from_db()
        self.assertEqual(prompt.tags, ["a", "b"])
        self.assertTrue(prompt.is_active)

    def test_tripleta_duplicada_falla(self):
        make_prompt("dup", version=1)
        with self.assertRaises(services.PromptError):
            services.create_prompt(slug="dup", body="x", version=1)

    def test_mismo_slug_otra_version_ok(self):
        make_prompt("ver", version=1)
        prompt = services.create_prompt(slug="ver", body="v2", version=2)
        self.assertIsNotNone(prompt.pk)
        self.assertEqual(Prompt.objects.filter(slug="ver").count(), 2)

    def test_mismo_slug_otro_proyecto_ok(self):
        make_prompt("shared", version=1)  # project_id NULL
        other_project = uuid.uuid4()
        prompt = services.create_prompt(
            project_id=other_project, slug="shared", body="x", version=1,
        )
        self.assertIsNotNone(prompt.pk)
        self.assertEqual(Prompt.objects.filter(slug="shared").count(), 2)


class UpdatePromptTests(TestCase):
    def test_actualiza_campos(self):
        p = make_prompt("viejo", body="antes")
        services.update_prompt(
            p, slug="viejo", body="despues", version=1,
            description="nueva desc", is_active=False, tags=["x"],
        )
        p.refresh_from_db()
        self.assertEqual(p.body, "despues")
        self.assertEqual(p.description, "nueva desc")
        self.assertFalse(p.is_active)
        self.assertEqual(p.tags, ["x"])

    def test_cambia_version_ok(self):
        p = make_prompt("v", version=1)
        services.update_prompt(p, slug="v", body="x", version=3, is_active=True)
        p.refresh_from_db()
        self.assertEqual(p.version, 3)

    def test_cuadrupla_choca_con_otro_falla(self):
        make_prompt("ocupado", version=2)
        p = make_prompt("ocupado", version=1)
        with self.assertRaises(services.PromptError):
            services.update_prompt(p, slug="ocupado", body="x", version=2)

    def test_mantener_su_propia_cuadrupla_no_choca(self):
        p = make_prompt("estable", version=1, body="original")
        services.update_prompt(p, slug="estable", body="editado", version=1)
        p.refresh_from_db()
        self.assertEqual(p.body, "editado")


class DeletePromptTests(TestCase):
    def test_soft_delete_no_borra_fila(self):
        p = make_prompt("borrar")
        services.delete_prompt(p)
        p.refresh_from_db()
        self.assertIsNotNone(p.deleted_at)
        self.assertFalse(p.is_active)
        self.assertTrue(Prompt.objects.filter(pk=p.pk).exists())


class ToggleStatusTests(TestCase):
    def test_active_a_inactive(self):
        p = make_prompt("t1", is_active=True)
        self.assertFalse(services.toggle_prompt_status(p))

    def test_inactive_a_active(self):
        p = make_prompt("t2", is_active=False)
        self.assertTrue(services.toggle_prompt_status(p))

    def test_soft_deleted_se_reactiva(self):
        p = make_prompt("t3", deleted=True)
        new_active = services.toggle_prompt_status(p)
        self.assertTrue(new_active)
        p.refresh_from_db()
        self.assertTrue(p.is_active)
        self.assertIsNone(p.deleted_at)

    def test_persiste_en_db(self):
        p = make_prompt("t4", is_active=True)
        services.toggle_prompt_status(p)
        p.refresh_from_db()
        self.assertFalse(p.is_active)


class ListSignalTests(TestCase):
    def test_signal_cuenta_prompts(self):
        make_prompt("s1")
        make_prompt("s2")
        sig = services.get_list_signal()
        self.assertEqual(sig["count"], 2)
        self.assertTrue(sig["version"])

    def test_signal_cambia_tras_modificacion(self):
        p = make_prompt("s3", is_active=True)
        before = services.get_list_signal()
        services.toggle_prompt_status(p)
        after = services.get_list_signal()
        self.assertNotEqual(before["version"], after["version"])

    def test_signal_cambia_tras_alta(self):
        before = services.get_list_signal()
        make_prompt("s4")
        after = services.get_list_signal()
        self.assertEqual(after["count"], before["count"] + 1)


class StatsTests(TestCase):
    def test_stats_cuenta_por_estado(self):
        make_prompt("a", is_active=True)
        make_prompt("b", is_active=True)
        make_prompt("c", is_active=False)
        make_prompt("d", deleted=True)
        stats = services.get_stats()
        self.assertEqual(stats["total"], 3)  # el borrado no cuenta
        self.assertEqual(stats["active"], 2)
        self.assertEqual(stats["inactive"], 1)
