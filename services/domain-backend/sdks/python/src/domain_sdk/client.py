"""DomainClient: cliente async HTTP."""

from __future__ import annotations

import os
from types import TracebackType
from typing import Any, Self

import httpx

from .errors import (
    AuthError,
    ConflictError,
    DomainError,
    NotFoundError,
    QuotaExceededError,
    RateLimitError,
    ValidationError,
)
from .resources import (
    AgentsResource,
    FlowsResource,
    KnowledgeResource,
    ObservationsResource,
    OrganizationsResource,
    ProjectsResource,
    SearchResource,
    SessionsResource,
    SkillsResource,
)


class DomainClient:
    """Cliente async para Domain API.

    Ejemplo::

        async with DomainClient(api_key="domk_live_...", base_url="https://...") as c:
            obs = await c.observations.save(project_slug="demo", content="hola")
    """

    def __init__(
        self,
        *,
        api_key: str | None = None,
        base_url: str | None = None,
        timeout: float = 30.0,
        http_client: httpx.AsyncClient | None = None,
    ) -> None:
        self.api_key = api_key or os.environ.get("DOMAIN_API_KEY", "")
        self.base_url = (base_url or os.environ.get("DOMAIN_BASE_URL")
                         or "http://localhost:8000").rstrip("/")
        if not self.api_key:
            raise AuthError("api_key required (pass arg or set DOMAIN_API_KEY env)")

        self._http = http_client or httpx.AsyncClient(
            timeout=timeout,
            headers={
                "Authorization": f"Bearer {self.api_key}",
                "Content-Type": "application/json",
                "User-Agent": "domain-sdk-python/0.1.0",
            },
        )
        self._owns_http = http_client is None

        # Resources
        self.organizations = OrganizationsResource(self)
        self.projects = ProjectsResource(self)
        self.observations = ObservationsResource(self)
        self.sessions = SessionsResource(self)
        self.search = SearchResource(self)
        self.skills = SkillsResource(self)
        self.agents = AgentsResource(self)
        self.flows = FlowsResource(self)
        self.knowledge = KnowledgeResource(self)

    async def __aenter__(self) -> Self:
        return self

    async def __aexit__(
        self,
        exc_type: type[BaseException] | None,
        exc: BaseException | None,
        tb: TracebackType | None,
    ) -> None:
        await self.close()

    async def close(self) -> None:
        if self._owns_http:
            await self._http.aclose()

    async def request(
        self,
        method: str,
        path: str,
        *,
        json: Any | None = None,
        params: dict[str, Any] | None = None,
    ) -> Any:
        """Llama bajo /api/v1 + path, parsea response.data y traduce errores."""
        url = f"{self.base_url}/api/v1{path}"
        resp = await self._http.request(method, url, json=json, params=params)
        if resp.status_code == 204:
            return None
        try:
            body = resp.json()
        except ValueError:
            body = {}
        if 200 <= resp.status_code < 300:
            return body.get("data", body)
        # Traducir error según code
        err = body.get("error", {})
        msg = err.get("message", resp.text or "error")
        code = err.get("code", "")
        rid = err.get("request_id", resp.headers.get("X-Request-Id", ""))
        details = err.get("details", [])
        kw = dict(message=msg, code=code, status=resp.status_code,
                  request_id=rid, details=details)
        match resp.status_code:
            case 401 | 403:
                raise AuthError(**kw)
            case 404:
                raise NotFoundError(**kw)
            case 409:
                raise ConflictError(**kw)
            case 422:
                raise ValidationError(**kw)
            case 402:
                raise QuotaExceededError(**kw)
            case 429:
                ra = float(resp.headers.get("Retry-After", "0") or 0)
                raise RateLimitError(retry_after=ra, **kw)
            case _:
                raise DomainError(**kw)
