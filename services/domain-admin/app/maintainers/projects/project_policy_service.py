from __future__ import annotations

from django.core.paginator import Paginator
from django.db import connection

from .models import Project, ProjectPolicy


class ProjectPolicyService:
    def list_platform_policies(self) -> list[dict]:
        with connection.cursor() as cur:
            cur.execute(
                "SELECT name, COALESCE(kind,''), COALESCE(body_md,'') "
                "FROM platform_policies WHERE is_active = TRUE ORDER BY kind, slug"
            )
            return [{"name": r[0], "kind": r[1], "body_md": r[2]} for r in cur.fetchall()]

    def list_project_rules(
        self, project: Project, scope: str = "all", page: int = 1, per_page: int = 10
    ) -> dict:
        platform = [
            {"scope": "platform", "kind": d["kind"], "name": d["name"], "id": None,
             "is_active": True, "override_platform": False, "editable": False}
            for d in self.list_platform_policies()
        ]
        project_rules = [
            {"scope": "project", "kind": p.kind, "name": p.name, "id": str(p.id),
             "is_active": p.is_active, "override_platform": p.override_platform, "editable": True}
            for p in ProjectPolicy.objects.filter(
                project_id=project.id, deleted_at__isnull=True
            ).order_by("kind", "slug")
        ]
        combined = platform + project_rules

        filtered = platform if scope == "platform" else project_rules if scope == "project" else combined
        paginator = Paginator(filtered, per_page)
        page = max(1, min(int(page or 1), paginator.num_pages or 1))
        pg = paginator.page(page)
        return {
            "items": list(pg.object_list),
            "total": paginator.count,
            "page": page,
            "per_page": per_page,
            "total_pages": paginator.num_pages,
            "has_prev": pg.has_previous(),
            "has_next": pg.has_next(),
            "scope": scope,
            "platform_count": len(platform),
            "project_count": len(project_rules),
        }

    def toggle_project_policy(self, project: Project, policy_id: str) -> None:
        from maintainers.projectpolicies import services as pp_services

        policy = ProjectPolicy.objects.filter(
            project_id=project.id, pk=policy_id, deleted_at__isnull=True
        ).first()
        if policy:
            pp_services.toggle_policy_status(policy)


_policy_service = ProjectPolicyService()


def get_policy_service() -> ProjectPolicyService:
    return _policy_service
