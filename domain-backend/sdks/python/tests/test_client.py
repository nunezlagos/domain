"""Tests del SDK Python con respx mocking."""

from __future__ import annotations

import pytest
import respx
from httpx import Response

from domain_sdk import DomainClient, AuthError, NotFoundError, ConflictError, QuotaExceededError


@pytest.fixture
def client_factory():
    def _factory(api_key: str = "domk_test_abcdefg"):
        return DomainClient(api_key=api_key, base_url="http://test.local")
    return _factory


@respx.mock
async def test_create_project(client_factory):
    respx.post("http://test.local/api/v1/projects").mock(
        return_value=Response(201, json={"data": {
            "id": "11111111-1111-1111-1111-111111111111",
            "organization_id": "22222222-2222-2222-2222-222222222222",
            "name": "Demo", "slug": "demo", "description": "",
            "created_at": "2026-06-07T00:00:00Z",
        }}),
    )
    async with client_factory() as c:
        proj = await c.projects.create(name="Demo", slug="demo")
        assert proj.slug == "demo"


@respx.mock
async def test_observations_save(client_factory):
    respx.post("http://test.local/api/v1/observations").mock(
        return_value=Response(201, json={"data": {
            "id": "33333333-3333-3333-3333-333333333333",
            "organization_id": "22222222-2222-2222-2222-222222222222",
            "project_id": "11111111-1111-1111-1111-111111111111",
            "content": "hola", "observation_type": "note",
            "tags": [], "metadata": {},
            "created_at": "2026-06-07T00:00:00Z",
        }}),
    )
    async with client_factory() as c:
        obs = await c.observations.save(project_slug="demo", content="hola")
        assert obs.content == "hola"


@respx.mock
async def test_auth_error(client_factory):
    respx.post("http://test.local/api/v1/projects").mock(
        return_value=Response(401, json={
            "error": {"code": "unauthorized", "message": "unauthorized"},
        }),
    )
    async with client_factory() as c:
        with pytest.raises(AuthError):
            await c.projects.create(name="X", slug="x")


@respx.mock
async def test_not_found_error(client_factory):
    respx.get("http://test.local/api/v1/projects/no-existe").mock(
        return_value=Response(404, json={
            "error": {"code": "not_found", "message": "project not found"},
        }),
    )
    async with client_factory() as c:
        with pytest.raises(NotFoundError):
            await c.projects.get("no-existe")


@respx.mock
async def test_conflict_error(client_factory):
    respx.post("http://test.local/api/v1/projects").mock(
        return_value=Response(409, json={
            "error": {"code": "slug_taken", "message": "slug already exists"},
        }),
    )
    async with client_factory() as c:
        with pytest.raises(ConflictError) as ei:
            await c.projects.create(name="X", slug="dup")
        assert ei.value.code == "slug_taken"


@respx.mock
async def test_quota_exceeded(client_factory):
    respx.post("http://test.local/api/v1/agents/aaa/run").mock(
        return_value=Response(402, json={
            "error": {"code": "quota_exceeded", "message": "monthly tokens exhausted"},
        }),
    )
    async with client_factory() as c:
        with pytest.raises(QuotaExceededError):
            await c.agents.run("aaa", input="hola")


def test_requires_api_key():
    with pytest.raises(AuthError):
        # sin DOMAIN_API_KEY env y sin arg
        DomainClient(api_key="", base_url="http://x")
