"""Tests de la gestion de skills por proyecto (project_skills)."""
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

    def test_disponibles_lista_globales_no_enlazadas(self):
        avail = services.list_available_skills(self.project)
        slugs = {s.slug for s in avail}
        self.assertIn("g1", slugs)
        self.assertIn("g2", slugs)
        self.assertNotIn("gx", slugs)  # soft-deleted excluida

    def test_link_y_unlink(self):
        services.link_skill(self.project, str(self.g1.id))
        linked = services.list_linked_skills(self.project)
        self.assertEqual([s.slug for s in linked], ["g1"])
        # ya enlazada -> no aparece en disponibles
        self.assertNotIn("g1", {s.slug for s in services.list_available_skills(self.project)})
        # idempotente
        services.link_skill(self.project, str(self.g1.id))
        self.assertEqual(ProjectSkill.objects.filter(project=self.project).count(), 1)
        # unlink
        services.unlink_skill(self.project, str(self.g1.id))
        self.assertEqual(services.list_linked_skills(self.project), [])


class ProjectSkillsViewTests(MaintainerTestCase):
    def setUp(self):
        self.authenticate()
        self.project = make_project("Demo", slug="demo")
        self.g1 = make_skill("Global 1", slug="g1")

    def test_modal_get(self):
        r = self.client.get(reverse("projects:manage_skills", args=[self.project.pk]))
        self.assertEqual(r.status_code, 200)
        self.assertContains(r, "Skills de")
        self.assertContains(r, "g1")

    def test_toggle_link_then_unlink(self):
        url = reverse("projects:toggle_skill", args=[self.project.pk])
        r = self.client.post(url, {"skill_id": str(self.g1.id), "op": "link"})
        self.assertEqual(r.status_code, 200)
        self.assertTrue(ProjectSkill.objects.filter(project=self.project, skill_id=self.g1.id).exists())
        r = self.client.post(url, {"skill_id": str(self.g1.id), "op": "unlink"})
        self.assertEqual(r.status_code, 200)
        self.assertFalse(ProjectSkill.objects.filter(project=self.project, skill_id=self.g1.id).exists())

    def test_toggle_requiere_auth(self):
        from django.test import Client
        anon = Client()  # sin authenticate() -> sin flag de sesion
        r = anon.post(reverse("projects:toggle_skill", args=[self.project.pk]),
                      {"skill_id": str(self.g1.id), "op": "link"})
        self.assertEqual(r.status_code, 302)
