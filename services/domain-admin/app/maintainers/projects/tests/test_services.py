"""Tests de la capa de servicio del mantenedor de Proyectos.

Corren contra Postgres real (managed=True via core.tests.runner). Cada assert
verifica un efecto observable en la DB o el valor de retorno real, no un mock.
"""
from __future__ import annotations

from core.tests.base import MaintainerTestCase

from maintainers.projects import services
from maintainers.projects.models import Project

from .factories import make_project, make_repository, make_template


class ListProjectsTests(MaintainerTestCase):
    def setUp(self):
        make_project("Alpha", slug="alpha", description="primero")
        make_project("Beta", slug="beta", repository_url="https://github.com/x/beta")
        make_project("Gamma", slug="gamma")

    def test_sin_search_devuelve_todos_activos(self):
        data = services.list_projects(search="", page=1, per_page=20)
        self.assertEqual(data["total"], 3)
        self.assertEqual(len(data["projects"]), 3)
        self.assertEqual(data["total_pages"], 1)
        self.assertFalse(data["has_next"])
        self.assertFalse(data["has_prev"])

    def test_excluye_archivados(self):
        make_project("Borrado", slug="borrado", archived=True)
        data = services.list_projects(search="", page=1, per_page=20)
        self.assertEqual(data["total"], 3)

    def test_search_por_slug(self):
        data = services.list_projects(search="beta", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["projects"][0].slug, "beta")

    def test_search_por_descripcion(self):
        data = services.list_projects(search="primero", page=1, per_page=20)
        self.assertEqual(data["total"], 1)
        self.assertEqual(data["projects"][0].slug, "alpha")

    def test_search_por_repo(self):
        data = services.list_projects(search="github.com/x/beta", page=1, per_page=20)
        self.assertEqual(data["total"], 1)

    def test_search_sin_match(self):
        data = services.list_projects(search="zzz-no-existe", page=1, per_page=20)
        self.assertEqual(data["total"], 0)
        self.assertEqual(data["projects"], [])

    def test_paginacion_parte_la_lista(self):
        p1 = services.list_projects(search="", page=1, per_page=2)
        self.assertEqual(p1["total"], 3)
        self.assertEqual(len(p1["projects"]), 2)
        self.assertEqual(p1["total_pages"], 2)
        self.assertTrue(p1["has_next"])
        self.assertFalse(p1["has_prev"])

        p2 = services.list_projects(search="", page=2, per_page=2)
        self.assertEqual(len(p2["projects"]), 1)
        self.assertFalse(p2["has_next"])
        self.assertTrue(p2["has_prev"])


class CreateProjectTests(MaintainerTestCase):
    def test_crea_proyecto_ok(self):
        project = services.create_project(name="Nuevo", slug="nuevo")
        self.assertIsNotNone(project.pk)
        self.assertTrue(Project.objects.filter(slug="nuevo").exists())

    def test_slug_duplicado_falla(self):
        make_project("Dup", slug="dup")
        with self.assertRaises(services.ProjectError):
            services.create_project(name="Otro", slug="dup")

    def test_template_inexistente_falla(self):
        import uuid
        with self.assertRaises(services.ProjectError):
            services.create_project(
                name="X", slug="x", template_id=str(uuid.uuid4()),
            )

    def test_template_valido_ok(self):
        tpl = make_template("django")
        project = services.create_project(
            name="Con tpl", slug="con-tpl", template_id=str(tpl.pk),
        )
        # template_id es UUIDField; recien coacciona a UUID tras round-trip a BD.
        project.refresh_from_db()
        self.assertEqual(project.template_id, tpl.pk)


class UpdateProjectTests(MaintainerTestCase):
    def test_actualiza_campos(self):
        p = make_project("Viejo", slug="viejo")
        services.update_project(
            p, name="Nuevo Nombre", slug="viejo", description="cambiada",
            repository_url="https://x/y", current_branch="develop",
        )
        p.refresh_from_db()
        self.assertEqual(p.name, "Nuevo Nombre")
        self.assertEqual(p.description, "cambiada")
        self.assertEqual(p.current_branch, "develop")

    def test_cambiar_slug_a_uno_ocupado_falla(self):
        make_project("Ocupado", slug="ocupado")
        p = make_project("Mio", slug="mio")
        with self.assertRaises(services.ProjectError):
            services.update_project(p, name="Mio", slug="ocupado")

    def test_mantener_su_propio_slug_ok(self):
        p = make_project("Mio", slug="mio")
        services.update_project(p, name="Mio v2", slug="mio")
        p.refresh_from_db()
        self.assertEqual(p.name, "Mio v2")


class DeleteProjectTests(MaintainerTestCase):
    def test_soft_delete_no_borra_fila(self):
        p = make_project("Borrar", slug="borrar")
        services.delete_project(p)
        p.refresh_from_db()
        self.assertIsNotNone(p.deleted_at)
        self.assertEqual(p.status, Project.STATUS_ARCHIVED)
        self.assertTrue(Project.objects.filter(pk=p.pk).exists())


class ToggleProjectTests(MaintainerTestCase):
    def test_activo_a_archivado(self):
        p = make_project("T", slug="t")
        self.assertEqual(services.toggle_project_status(p), Project.STATUS_ARCHIVED)
        self.assertIsNotNone(p.deleted_at)

    def test_archivado_a_activo(self):
        p = make_project("T", slug="t", archived=True)
        self.assertEqual(services.toggle_project_status(p), Project.STATUS_ACTIVE)
        self.assertIsNone(p.deleted_at)

    def test_persiste_en_db(self):
        p = make_project("T", slug="t")
        services.toggle_project_status(p)
        p.refresh_from_db()
        self.assertIsNotNone(p.deleted_at)


class RepositoriesTests(MaintainerTestCase):
    def test_lista_solo_activos_default_primero(self):
        p = make_project("Repo", slug="repo")
        make_repository(p, "mirror", is_default=False)
        make_repository(p, "origin", is_default=True)
        make_repository(p, "borrado", deleted=True)
        repos = services.get_project_repositories(p)
        self.assertEqual(len(repos), 2)
        self.assertTrue(repos[0].is_default)
        self.assertEqual(repos[0].name, "origin")


class SyncRepositoriesTests(MaintainerTestCase):
    def test_create_con_repos_setea_default_y_url_principal(self):
        project = services.create_project(
            name="P", slug="p",
            repositories=[
                {"url": "https://github.com/org/domain", "branch_default": "main", "root_path": "/"},
                {"url": "https://gitlab.com/org/mirror", "branch_default": "", "root_path": "/sub/"},
            ],
        )
        repos = services.get_project_repositories(project)
        self.assertEqual(len(repos), 2)
        # default = primero; name derivado de la URL.
        self.assertTrue(repos[0].is_default)
        self.assertEqual(repos[0].name, "domain")
        self.assertEqual(repos[0].root_path, "/")
        # URL principal legacy = repo default.
        project.refresh_from_db()
        self.assertEqual(project.repository_url, "https://github.com/org/domain")

    def test_repositories_none_no_toca_repos(self):
        project = services.create_project(
            name="P", slug="p", repository_url="https://x/y",
        )
        self.assertEqual(len(services.get_project_repositories(project)), 0)
        project.refresh_from_db()
        self.assertEqual(project.repository_url, "https://x/y")

    def test_update_reemplaza_set_y_softdeletea_sobrantes(self):
        project = services.create_project(
            name="P", slug="p",
            repositories=[
                {"url": "https://github.com/org/a", "branch_default": "", "root_path": ""},
                {"url": "https://github.com/org/b", "branch_default": "", "root_path": ""},
            ],
        )
        services.update_project(
            project, name="P", slug="p",
            repositories=[{"url": "https://github.com/org/c", "branch_default": "dev", "root_path": "/c/"}],
        )
        repos = services.get_project_repositories(project)
        self.assertEqual(len(repos), 1)
        self.assertEqual(repos[0].url, "https://github.com/org/c")
        self.assertEqual(repos[0].root_path, "/c/")
        self.assertTrue(repos[0].is_default)
        project.refresh_from_db()
        self.assertEqual(project.repository_url, "https://github.com/org/c")

    def test_update_con_lista_vacia_limpia_todo(self):
        project = services.create_project(
            name="P", slug="p",
            repositories=[{"url": "https://github.com/org/a", "branch_default": "", "root_path": ""}],
        )
        services.update_project(project, name="P", slug="p", repositories=[])
        self.assertEqual(len(services.get_project_repositories(project)), 0)
        project.refresh_from_db()
        self.assertEqual(project.repository_url, "")

    def test_update_repositories_none_preserva_repos(self):
        project = services.create_project(
            name="P", slug="p",
            repositories=[{"url": "https://github.com/org/a", "branch_default": "", "root_path": ""}],
        )
        services.update_project(project, name="P2", slug="p")  # sin repositories
        self.assertEqual(len(services.get_project_repositories(project)), 1)


class TemplatesTests(MaintainerTestCase):
    def test_lista_todos_ordenados_por_slug(self):
        make_template("zeta")
        make_template("alpha", is_public=True)
        make_template("mid")
        tpls = services.list_available_templates()
        slugs = [t.slug for t in tpls]
        self.assertEqual(slugs, ["alpha", "mid", "zeta"])


class ListSignalTests(MaintainerTestCase):
    def test_signal_cuenta_proyectos(self):
        make_project("S1", slug="s1")
        make_project("S2", slug="s2")
        sig = services.get_list_signal()
        self.assertEqual(sig["count"], 2)
        self.assertTrue(sig["version"])

    def test_signal_cambia_tras_alta(self):
        before = services.get_list_signal()
        make_project("S3", slug="s3")
        after = services.get_list_signal()
        self.assertEqual(after["count"], before["count"] + 1)

    def test_signal_cambia_tras_modificacion(self):
        p = make_project("S4", slug="s4")
        before = services.get_list_signal()
        services.toggle_project_status(p)  # bump updated_at + deleted_at
        after = services.get_list_signal()
        self.assertNotEqual(before["version"], after["version"])


class StatsTests(MaintainerTestCase):
    def test_cuenta_activos_y_archivados(self):
        make_project("A", slug="a")
        make_project("B", slug="b")
        make_project("C", slug="c", archived=True)
        stats = services.get_stats()
        self.assertEqual(stats["active"], 2)
        self.assertEqual(stats["archived"], 1)
        self.assertEqual(stats["total"], 3)
