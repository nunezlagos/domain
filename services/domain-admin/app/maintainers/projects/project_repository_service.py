from __future__ import annotations

import re

from django.utils import timezone

from .models import Project, ProjectRepository


class ProjectRepositoryService:
    def get_project_repositories(self, project: Project) -> list[ProjectRepository]:
        return list(
            ProjectRepository.objects.filter(project=project, deleted_at__isnull=True)
            .order_by("-is_default", "name")
        )

    def sync_repositories(self, project: Project, rows: list[dict]) -> None:
        existing = list(
            ProjectRepository.objects.filter(
                project=project, deleted_at__isnull=True
            ).order_by("created_at", "id")
        )

        for i, row in enumerate(rows):
            name = self._derive_repo_name(row["url"], i)
            is_default = i == 0
            if i < len(existing):
                repo = existing[i]
                repo.name = name
                repo.url = row["url"]
                repo.branch_default = row.get("branch_default", "")
                repo.root_path = row.get("root_path", "")
                repo.is_default = is_default
                repo.save()
            else:
                ProjectRepository.objects.create(
                    project=project,
                    name=name,
                    url=row["url"],
                    branch_default=row.get("branch_default", ""),
                    root_path=row.get("root_path", ""),
                    is_default=is_default,
                )

        for repo in existing[len(rows):]:
            repo.deleted_at = timezone.now()
            repo.is_default = False
            repo.save()

        project.repository_url = rows[0]["url"] if rows else ""
        project.save(update_fields=["repository_url", "updated_at"])

    def _derive_repo_name(self, url: str, index: int) -> str:
        base = url.rstrip("/").rsplit("/", 1)[-1]
        if base.endswith(".git"):
            base = base[:-4]
        base = re.sub(r"[^A-Za-z0-9._-]", "", base).strip("-")
        if base:
            return base[:50]
        return "origin" if index == 0 else f"repo-{index + 1}"


_repository_service = ProjectRepositoryService()


def get_repository_service() -> ProjectRepositoryService:
    return _repository_service
