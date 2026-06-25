from __future__ import annotations

from django.core.paginator import Paginator

from .models import Project, ProjectSkill


class ProjectSkillService:
    def _excluded_skill_ids(self, project: Project) -> set:
        return set(
            ProjectSkill.objects.filter(project=project, is_enabled=False)
            .values_list("skill_id", flat=True)
        )

    def list_project_skills(
        self, project: Project, scope: str = "all", page: int = 1, per_page: int = 10
    ) -> dict:
        from maintainers.skills.models import Skill

        excluded = self._excluded_skill_ids(project)
        globals_ = list(
            Skill.objects.filter(project_id__isnull=True, deleted_at__isnull=True).order_by("slug")
        )
        internals = list(
            Skill.objects.filter(project_id=project.id, deleted_at__isnull=True).order_by("slug")
        )
        for s in globals_:
            s.scope, s.excluded = "global", s.id in excluded
        for s in internals:
            s.scope, s.excluded = "internal", s.id in excluded
        combined = globals_ + internals

        filtered = globals_ if scope == "global" else internals if scope == "internal" else combined
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
            "applied_count": sum(1 for s in combined if not s.excluded),
            "excluded_count": sum(1 for s in combined if s.excluded),
            "global_count": len(globals_),
            "internal_count": len(internals),
        }

    def set_skill_excluded(self, project: Project, skill_id: str, excluded: bool) -> None:
        if excluded:
            ProjectSkill.objects.update_or_create(
                project=project, skill_id=skill_id, defaults={"is_enabled": False}
            )
        else:
            ProjectSkill.objects.filter(project=project, skill_id=skill_id).delete()


_skill_service = ProjectSkillService()


def get_skill_service() -> ProjectSkillService:
    return _skill_service
