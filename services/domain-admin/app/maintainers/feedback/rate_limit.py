"""HU-52.1: rate limit del feedback loop (anti-spam).

Reusa el `RateLimiter` (sliding window in-memory) ya existente en la app
chat para no duplicar logica. Limite: 30 req/min por user_email, que espeja
el limite del endpoint REST del Go (FeedbackLimiter, 30/min).
"""
from __future__ import annotations

from chat.rate_limit import RateLimitResult, RateLimiter

_feedback_limiter = RateLimiter(max_requests=30, window_seconds=60)


def check_feedback(key: str) -> RateLimitResult:
    """Chequea el rate limit del feedback (30 req/min por user_email)."""
    return _feedback_limiter.check(key)


def reset_feedback(key: str | None = None) -> None:
    """Resetea el rate limit (util en tests)."""
    _feedback_limiter.reset(key)
