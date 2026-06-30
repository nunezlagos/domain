"""Tests del rate limiter (LLM04 DoS protection)."""
from __future__ import annotations

import time

import pytest

from chat.rate_limit import RateLimitResult, RateLimiter, check_default, reset_default


def test_rate_limiter_pasa_primera_request():
    limiter = RateLimiter(max_requests=3, window_seconds=1)
    result = limiter.check("user1")
    assert result.allowed is True
    assert result.remaining == 2


def test_rate_limiter_bloquea_despues_de_max():
    limiter = RateLimiter(max_requests=3, window_seconds=1)
    for _ in range(3):
        assert limiter.check("user1").allowed is True
    result = limiter.check("user1")
    assert result.allowed is False
    assert result.remaining == 0
    assert result.reset_in_seconds >= 1


def test_rate_limiter_keys_independientes():
    limiter = RateLimiter(max_requests=2, window_seconds=1)
    assert limiter.check("user1").allowed is True
    assert limiter.check("user1").allowed is True
    assert limiter.check("user1").allowed is False
    assert limiter.check("user2").allowed is True


def test_rate_limiter_reset_despues_de_ventana():
    limiter = RateLimiter(max_requests=2, window_seconds=0.5)
    assert limiter.check("user1").allowed is True
    assert limiter.check("user1").allowed is True
    assert limiter.check("user1").allowed is False
    time.sleep(0.6)
    assert limiter.check("user1").allowed is True


def test_rate_limiter_reset_una_key():
    limiter = RateLimiter(max_requests=2, window_seconds=10)
    for _ in range(2):
        limiter.check("user1")
    assert limiter.check("user1").allowed is False
    limiter.reset("user1")
    assert limiter.check("user1").allowed is True


def test_rate_limiter_reset_todas():
    limiter = RateLimiter(max_requests=1, window_seconds=10)
    limiter.check("user1")
    limiter.check("user2")
    limiter.reset()
    assert limiter.check("user1").allowed is True
    assert limiter.check("user2").allowed is True


def test_check_default_funciona():
    reset_default()
    for _ in range(10):
        result = check_default("test_user")
        assert result.allowed is True
    blocked = check_default("test_user")
    assert blocked.allowed is False
    reset_default("test_user")