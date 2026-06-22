"""Tests de la capa de servicio del mantenedor de Reglas por proyecto."""
from __future__ import annotations

import uuid

from core.tests.base import MaintainerTestCase

from maintainers.projectpolicies import services
from maintainers.projectpolicies.models import ProjectPolicy

from .factories import make_policy


class CreatePolicyTests(MaintainerTestCase):
    def test_crea_ok(self):
        pid = uuid.uuid4()
        p = services.create_policy(
            project_id=pid, slug="commits", name="Commits", kind="convention",
            body_md="Commits en español.",
        )
        self.assertIsNotNone(p.pk)
        self.assertEqual(p.project_id, pid)
        self.assertEqual(p.source, "dashboard")

    def test_sin_proyecto_falla(self):
        with self.assertRaises(services.ProjectPolicyError):
            services.create_policy(
                project_id=None, slug="x", name="X", kind="convention", body_md="b",
            )

    def test_slug_duplicado_en_proyecto_falla(self):
        pid = uuid.uuid4()
        make_policy("Dup", project_id=pid, slug="dup")
        with self.assertRaises(services.ProjectPolicyError):
            services.create_policy(
                project_id=pid, slug="dup", name="Otra", kind="convention", body_md="b",
            )

    def test_mismo_slug_distinto_proyecto_ok(self):
        make_policy("A", project_id=uuid.uuid4(), slug="regla")
        p = services.create_policy(
            project_id=uuid.uuid4(), slug="regla", name="B", kind="convention", body_md="b",
        )
        self.assertIsNotNone(p.pk)


class UpdateAndToggleTests(MaintainerTestCase):
    def test_update_cambia_cuerpo(self):
        p = make_policy("R", slug="r")
        services.update_policy(p, slug="r", name="R2", kind="security_rule",
                               body_md="nuevo", override_platform=True, is_active=True)
        p.refresh_from_db()
        self.assertEqual(p.name, "R2")
        self.assertEqual(p.kind, "security_rule")
        self.assertTrue(p.override_platform)

    def test_toggle_desactiva_y_reactiva(self):
        p = make_policy("R", slug="r", is_active=True)
        self.assertFalse(services.toggle_policy_status(p))
        self.assertTrue(services.toggle_policy_status(p))

    def test_delete_soft(self):
        p = make_policy("R", slug="r")
        services.delete_policy(p)
        p.refresh_from_db()
        self.assertIsNotNone(p.deleted_at)
        self.assertFalse(p.is_active)
        self.assertTrue(ProjectPolicy.objects.filter(pk=p.pk).exists())


class ListTests(MaintainerTestCase):
    def test_excluye_soft_deleted(self):
        make_policy("A", slug="a")
        make_policy("B", slug="b", deleted=True)
        data = services.list_policies(search="", page=1, per_page=20)
        self.assertEqual(data["total"], 1)

    def test_search_por_nombre(self):
        make_policy("Seguridad API", slug="sec")
        make_policy("Convencion", slug="conv")
        data = services.list_policies(search="Seguridad", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
