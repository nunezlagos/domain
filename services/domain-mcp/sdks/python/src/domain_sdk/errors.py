"""Excepciones tipadas según response shape de Domain (rules/api.md)."""

from __future__ import annotations


class DomainError(Exception):
    """Base error con campos de la response standard."""

    def __init__(
        self,
        message: str,
        code: str = "",
        status: int = 0,
        request_id: str = "",
        details: list[dict] | None = None,
    ) -> None:
        super().__init__(message)
        self.code = code
        self.status = status
        self.request_id = request_id
        self.details = details or []


class AuthError(DomainError):
    """401 unauthorized."""


class NotFoundError(DomainError):
    """404 not_found."""


class ValidationError(DomainError):
    """422 validation_failed con details campo-a-campo."""


class ConflictError(DomainError):
    """409 conflict (slug_taken, already_pending)."""


class RateLimitError(DomainError):
    """429 con Retry-After."""

    def __init__(self, message: str, retry_after: float = 0.0, **kw: object) -> None:
        super().__init__(message, **kw)  # type: ignore[arg-type]
        self.retry_after = retry_after


class QuotaExceededError(DomainError):
    """402 quota_exceeded."""
