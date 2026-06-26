"""HU-49.2 extension: rate limit simple para evitar DoS (LLM04).

Limit: max N requests por minuto por usuario (identificado por email
de sesion). Si se excede, retorna 429.

Implementacion: in-memory counter con ventana deslizante. No usamos
Redis/Postgres para mantener la simplicidad — el admin es single-user
en la mayoria de los casos. Si en el futuro se necesita multi-user
concurrent, migrar a Redis con sliding window counters.
"""
from __future__ import annotations

import logging
import time
from collections import deque
from dataclasses import dataclass

log = logging.getLogger(__name__)


@dataclass(frozen=True)
class RateLimitResult:
    allowed: bool
    remaining: int
    reset_in_seconds: int
    reason: str = ""


class RateLimiter:
    """In-memory sliding window rate limiter."""

    def __init__(self, max_requests: int = 10, window_seconds: int = 60) -> None:
        self._max = max_requests
        self._window = window_seconds
        self._buckets: dict[str, deque[float]] = {}

    def check(self, key: str) -> RateLimitResult:
        """Chequea si `key` puede hacer una request.

        Args:
            key: identificador unico del usuario (ej: email de sesion).

        Returns:
            RateLimitResult con allowed=True si esta dentro del limite.
        """
        now = time.monotonic()
        bucket = self._buckets.setdefault(key, deque())
        cutoff = now - self._window
        while bucket and bucket[0] < cutoff:
            bucket.popleft()

        if len(bucket) >= self._max:
            oldest = bucket[0]
            reset_in = max(1, int(self._window - (now - oldest)))
            log.warning("rate limit hit for key=%s, reset in %ds", key, reset_in)
            return RateLimitResult(
                allowed=False,
                remaining=0,
                reset_in_seconds=reset_in,
                reason=f"rate limit: max {self._max} requests per {self._window}s",
            )

        bucket.append(now)
        return RateLimitResult(
            allowed=True,
            remaining=self._max - len(bucket),
            reset_in_seconds=self._window,
        )

    def reset(self, key: str | None = None) -> None:
        """Resetea el rate limit. Si key es None, resetea todo."""
        if key is None:
            self._buckets.clear()
        else:
            self._buckets.pop(key, None)


_default_limiter = RateLimiter(max_requests=10, window_seconds=60)


def check_default(key: str) -> RateLimitResult:
    """Chequea el rate limit default (10 req/min)."""
    return _default_limiter.check(key)


def reset_default(key: str | None = None) -> None:
    """Resetea el rate limit default."""
    _default_limiter.reset(key)