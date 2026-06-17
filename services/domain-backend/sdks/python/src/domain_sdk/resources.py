"""Resources del cliente: agrupan endpoints por entidad."""

from __future__ import annotations

from typing import TYPE_CHECKING, Any

from .models import (
    AgentRunResult,
    Observation,
    Organization,
    Project,
    SearchResult,
    Session,
)

if TYPE_CHECKING:
    from .client import DomainClient


class _Base:
    def __init__(self, client: "DomainClient") -> None:
        self._c = client


class OrganizationsResource(_Base):
    # single-org (issue-21.5): create/delete de orgs se removieron del backend.
    async def get(self, id: str) -> Organization:
        data = await self._c.request("GET", f"/organizations/{id}")
        return Organization.model_validate(data)

    async def list_members(self, org_id: str) -> list[dict[str, Any]]:
        return await self._c.request("GET", f"/organizations/{org_id}/members") or []


class ProjectsResource(_Base):
    async def create(
        self, *, name: str, slug: str, description: str = ""
    ) -> Project:
        data = await self._c.request(
            "POST", "/projects",
            json={"name": name, "slug": slug, "description": description},
        )
        return Project.model_validate(data)

    async def list(self) -> list[Project]:
        data = await self._c.request("GET", "/projects") or []
        return [Project.model_validate(d) for d in data]

    async def get(self, slug: str) -> Project:
        data = await self._c.request("GET", f"/projects/{slug}")
        return Project.model_validate(data)

    async def delete(self, slug: str) -> None:
        await self._c.request("DELETE", f"/projects/{slug}")


class ObservationsResource(_Base):
    async def save(
        self,
        *,
        project_slug: str,
        content: str,
        observation_type: str = "note",
        tags: list[str] | None = None,
        metadata: dict[str, Any] | None = None,
    ) -> Observation:
        data = await self._c.request(
            "POST", "/observations",
            json={
                "project_slug": project_slug, "content": content,
                "observation_type": observation_type,
                "tags": tags or [], "metadata": metadata or {},
            },
        )
        return Observation.model_validate(data)

    async def get(self, id: str) -> Observation:
        data = await self._c.request("GET", f"/observations/{id}")
        return Observation.model_validate(data)

    async def list(self, *, project_slug: str, limit: int = 50) -> list[Observation]:
        data = await self._c.request(
            "GET", "/observations",
            params={"project_slug": project_slug, "limit": limit},
        ) or []
        return [Observation.model_validate(d) for d in data]

    async def delete(self, id: str) -> None:
        await self._c.request("DELETE", f"/observations/{id}")


class SessionsResource(_Base):
    async def start(
        self, *, title: str, project_slug: str = "", tags: list[str] | None = None
    ) -> Session:
        body: dict[str, Any] = {"title": title, "tags": tags or []}
        if project_slug:
            body["project_slug"] = project_slug
        data = await self._c.request("POST", "/sessions", json=body)
        return Session.model_validate(data)

    async def end(self, session_id: str, *, summary: str = "") -> Session:
        data = await self._c.request(
            "POST", f"/sessions/{session_id}/end", json={"summary": summary}
        )
        return Session.model_validate(data)

    async def active(self, *, project_slug: str = "") -> Session | None:
        params = {"project_slug": project_slug} if project_slug else None
        data = await self._c.request("GET", "/sessions/active", params=params)
        if data is None:
            return None
        return Session.model_validate(data)


class SearchResource(_Base):
    async def global_(
        self,
        *,
        query: str,
        limit: int = 20,
        entity_types: list[str] | None = None,
        tags: list[str] | None = None,
    ) -> list[SearchResult]:
        params: dict[str, Any] = {"q": query, "limit": limit}
        if entity_types:
            params["entity_type"] = ",".join(entity_types)
        if tags:
            params["tags"] = ",".join(tags)
        data = await self._c.request("GET", "/search", params=params) or []
        return [SearchResult.model_validate(d) for d in data]


class SkillsResource(_Base):
    async def list(self, *, type: str = "", tag: str = "", limit: int = 50) -> list[dict[str, Any]]:
        params: dict[str, Any] = {"limit": limit}
        if type:
            params["type"] = type
        if tag:
            params["tag"] = tag
        return await self._c.request("GET", "/skills", params=params) or []

    async def create(self, **fields: Any) -> dict[str, Any]:
        return await self._c.request("POST", "/skills", json=fields)


class AgentsResource(_Base):
    async def list(self) -> list[dict[str, Any]]:
        return await self._c.request("GET", "/agents") or []

    async def get(self, id: str) -> dict[str, Any]:
        return await self._c.request("GET", f"/agents/{id}")

    async def run(self, agent_id: str, *, input: str,
                  variables: dict[str, Any] | None = None) -> AgentRunResult:
        data = await self._c.request(
            "POST", f"/agents/{agent_id}/run",
            json={"input": input, "variables": variables or {}},
        )
        return AgentRunResult.model_validate(data)

    async def run_logs(self, run_id: str) -> list[dict[str, Any]]:
        return await self._c.request("GET", f"/agent-runs/{run_id}/logs") or []


class FlowsResource(_Base):
    async def list(self) -> list[dict[str, Any]]:
        return await self._c.request("GET", "/flows") or []

    async def run(self, flow_id: str, *,
                  inputs: dict[str, Any] | None = None) -> dict[str, Any]:
        return await self._c.request(
            "POST", f"/flows/{flow_id}/run",
            json={"inputs": inputs or {}},
        )


class KnowledgeResource(_Base):
    async def save(
        self,
        *,
        project_slug: str,
        title: str,
        body: str,
        source: str = "manual",
        tags: list[str] | None = None,
    ) -> dict[str, Any]:
        return await self._c.request(
            "POST", "/knowledge",
            json={
                "project_slug": project_slug, "title": title, "body": body,
                "source": source, "tags": tags or [],
            },
        )

    async def search(self, query: str, *, limit: int = 20) -> list[dict[str, Any]]:
        return await self._c.request(
            "GET", "/knowledge/search", params={"q": query, "limit": limit},
        ) or []
