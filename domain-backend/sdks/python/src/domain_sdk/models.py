"""Modelos Pydantic que reflejan el shape de Domain API."""

from __future__ import annotations

from datetime import datetime
from typing import Any
from uuid import UUID

from pydantic import BaseModel, Field


class Organization(BaseModel):
    id: UUID
    name: str
    slug: str
    settings: dict[str, Any] = Field(default_factory=dict)
    created_at: datetime
    updated_at: datetime


class Project(BaseModel):
    id: UUID
    organization_id: UUID
    name: str
    slug: str
    description: str = ""
    created_at: datetime


class Observation(BaseModel):
    id: UUID
    organization_id: UUID
    project_id: UUID
    content: str
    observation_type: str = "note"
    tags: list[str] = Field(default_factory=list)
    metadata: dict[str, Any] = Field(default_factory=dict)
    created_at: datetime


class Session(BaseModel):
    id: UUID
    title: str
    summary: str = ""
    started_at: datetime
    ended_at: datetime | None = None


class SearchResult(BaseModel):
    entity_type: str
    id: UUID
    title: str = ""
    snippet: str = ""
    score: float = 0.0
    project_id: UUID | None = None
    tags: list[str] = Field(default_factory=list)
    created_at: datetime


class AgentRunResult(BaseModel):
    run_id: UUID
    status: str  # "completed" | "failed" | "running"
    output: str = ""
    error: str = ""
    tokens_input: int = 0
    tokens_output: int = 0
    iterations: int = 0
    started_at: datetime | None = None
    finished_at: datetime | None = None
