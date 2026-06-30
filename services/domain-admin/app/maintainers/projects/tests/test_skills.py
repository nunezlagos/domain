"""Tests del modelo hibrido de skills por proyecto (auto + excluibles).

Las skills globales (project_id NULL) aplican AUTOMATICAMENTE; project_skills se
usa SOLO para EXCLUIR (is_enabled=FALSE). Las internas (project_id = proyecto)
son propias del proyecto. El toggle excluye (op=exclude) / re-incluye (op=include).
"""
from __future__ import annotations

from django.urls import reverse

from core.tests.base import MaintainerTestCase

from maintainers.projects import services
from maintainers.projects.models import ProjectSkill
from maintainers.skills.tests.factories import make_skill

from .factories import make_project


class ProjectSkillsServiceTests(MaintainerTestCase):
    def setUp(self):
        self.project = make_project("Demo", slug="demo")
        self.g1 = make_skill("Global 1", slug="g1")          # global (project_id NULL)
        self.g2 = make_skill("Global 2", slug="g2")
        make_skill("Borrada", slug="gx", deleted=True)        # no debe aparecer

    def test_globales_aplican_automaticamente(self):
        data = services.list_project_skills(self.project)  # scope=all, page 1
        slugs = {s.slug for s in data["items"]}
        self.assertIn("g1", slugs)
        self.assertIn("g2", slugs)
        self.assertNotIn("gx", slugs)  # soft-deleted excluida
        self.assertFalse(any(s.excluded for s in data["items"]))
        self.assertEqual(data["excluded_count"], 0)
        self.assertEqual(data["global_count"], 2)

    def test_filtro_scope(self):
        only_internal = services.list_project_skills(self.project, scope="internal")
        self.assertEqual(only_internal["items"], [])  # no hay internas
        only_global = services.list_project_skills(self.project, scope="global")
        self.assertEqual({s.slug for s in only_global["items"]}, {"g1", "g2"})

    def test_excluir_y_reincluir(self):

        services.set_skill_excluded(self.project, str(self.g1.id), excluded=True)
        row = ProjectSkill.objects.get(project=self.project, skill_id=self.g1.id)
        self.assertFalse(row.is_enabled)
        data = services.list_project_skills(self.project)
        g1 = next(s for s in data["items"] if s.slug == "g1")
        self.assertTrue(g1.excluded)
        self.assertEqual(data["excluded_count"], 1)

        services.set_skill_excluded(self.project, str(self.g1.id), excluded=False)
        self.assertFalse(
            ProjectSkill.objects.filter(project=self.project, skill_id=self.g1.id).exists()
        )


class ProjectSkillsViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()
        self.project = make_project("Demo", slug="demo")
        self.g1 = make_skill("Global 1", slug="g1")

    def test_detalle_muestra_skills(self):

        r = self.client.get(reverse("projects:detail", args=[self.project.pk]) + "?partial=1")
        self.assertEqual(r.status_code, 200)
        self.assertContains(r, "g1")
        self.assertContains(r, "Globales")

    def test_toggle_exclude_then_include(self):
        url = reverse("projects:toggle_skill", args=[self.project.pk])
        r = self.client.post(url, {"skill_id": str(self.g1.id), "op": "exclude"})
        self.assertEqual(r.status_code, 200)
        row = ProjectSkill.objects.get(project=self.project, skill_id=self.g1.id)
        self.assertFalse(row.is_enabled)
        r = self.client.post(url, {"skill_id": str(self.g1.id), "op": "include"})
        self.assertEqual(r.status_code, 200)
        self.assertFalse(
            ProjectSkill.objects.filter(project=self.project, skill_id=self.g1.id).exists()
        )

    def test_toggle_requiere_auth(self):
        from django.test import Client
        anon = Client()  # sin authenticate() -> sin flag de sesion
        r = anon.post(reverse("projects:toggle_skill", args=[self.project.pk]),
                      {"skill_id": str(self.g1.id), "op": "exclude"})
        self.assertEqual(r.status_code, 302)
