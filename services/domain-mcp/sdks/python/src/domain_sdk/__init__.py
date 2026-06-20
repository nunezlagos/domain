"""Domain SDK Python — cliente oficial.

Usage:

    from domain_sdk import DomainClient

    async with DomainClient(api_key="domk_live_...") as client:
        obs = await client.observations.save(
            project_slug="demo",
            content="primera observación",
            tags=["test"],
        )
        results = await client.search.global_(query="primera")
"""

from .client import DomainClient
from .errors import (
    DomainError,
    AuthError,
    NotFoundError,
    ValidationError,
    ConflictError,
    RateLimitError,
    QuotaExceededError,
)
from .models import (
    Project,
    Observation,
    Session,
    SearchResult,
    AgentRunResult,
)

__version__ = "0.1.0"

__all__ = [
    "DomainClient",
    "DomainError",
    "AuthError",
    "NotFoundError",
    "ValidationError",
    "ConflictError",
    "RateLimitError",
    "QuotaExceededError",
    "Project",
    "Observation",
    "Session",
    "SearchResult",
    "AgentRunResult",
]
