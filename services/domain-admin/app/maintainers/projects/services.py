from __future__ import annotations

from .project_service import (
    ProjectService,
    ProjectError,
    list_projects,
    export_projects_csv,
    get_list_signal,
    get_project,
    list_available_templates,
    create_project,
    update_project,
    delete_project,
    toggle_project_status,
    get_stats,
    get_proyecto,
    create_proyecto,
    update_proyecto,
    delete_proyecto,
    toggle_proyecto_status,
    ServiceError,
)
from .project_repository_service import (
    ProjectRepositoryService,
    get_repository_service,
    get_project_repositories,
)
from .project_skill_service import (
    ProjectSkillService,
    get_skill_service,
    _skill_service,
)
from .project_policy_service import (
    ProjectPolicyService,
    get_policy_service,
    _policy_service,
)

__all__ = [
    "ProjectService",
    "ProjectError",
    "list_projects",
    "export_projects_csv",
    "get_list_signal",
    "get_project",
    "list_available_templates",
    "create_project",
    "update_project",
    "delete_project",
    "toggle_project_status",
    "get_stats",
    "get_proyecto",
    "create_proyecto",
    "update_proyecto",
    "delete_proyecto",
    "toggle_proyecto_status",
    "ServiceError",
    "ProjectRepositoryService",
    "get_repository_service",
    "get_project_repositories",
    "ProjectSkillService",
    "get_skill_service",
    "list_project_skills",
    "set_skill_excluded",
    "ProjectPolicyService",
    "get_policy_service",
    "list_project_rules",
    "toggle_project_policy",
]


def list_project_skills(project, scope="all", page=1, per_page=10):
    return _skill_service.list_project_skills(project, scope, page, per_page)


def set_skill_excluded(project, skill_id, excluded=True):
    return _skill_service.set_skill_excluded(project, skill_id, excluded)


def list_project_rules(project, scope="all", page=1, per_page=10):
    return _policy_service.list_project_rules(project, scope, page, per_page)


def toggle_project_policy(project, policy_id):
    return _policy_service.toggle_project_policy(project, policy_id)
