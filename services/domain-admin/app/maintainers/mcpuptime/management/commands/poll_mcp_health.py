"""Management command: poll_mcp_health.

Hace un GET al endpoint /health del server domain-mcp, mide la latencia y
registra una fila en `mcp_health_checks` con el estado resultante:

  - up:       HTTP 200 y el body reporta status "ok" (o no es parseable pero 200).
  - degraded: HTTP 2xx/3xx distinto de 200, o 200 con status != "ok"
              (responde pero no del todo sano).
  - down:     timeout, error de conexion, o HTTP >= 400.

Pensado para correr por cron del sistema o el scheduler del MCP, ej::

    */1 * * * *  python manage.py poll_mcp_health

La URL se resuelve de (en orden):
  1) flag --url
  2) settings.DOMAIN_MCP_HEALTH_URL (env DOMAIN_MCP_HEALTH_URL)
  3) settings.DOMAIN_BASE_URL + "/health" (env DOMAIN_BASE_URL)
  4) fallback: http://domain-mcp:8080/health

Usa stdlib urllib (sin dependencias extra). NUNCA levanta excepcion: cualquier
fallo se traduce en un registro 'down' con el error, para que la propia caida
del MCP quede medida.
"""
from __future__ import annotations

import json
import time
import urllib.error
import urllib.request

from django.conf import settings
from django.core.management.base import BaseCommand

from maintainers.mcpuptime.models import McpHealthCheck

_DEFAULT_URL = "http://domain-mcp:8080/health"
_DEFAULT_TIMEOUT = 5.0


def _resolve_url(cli_url: str | None) -> str:
    if cli_url:
        return cli_url
    explicit = getattr(settings, "DOMAIN_MCP_HEALTH_URL", "")
    if explicit:
        return explicit
    base = getattr(settings, "DOMAIN_BASE_URL", "")
    if base:
        return base.rstrip("/") + "/health"
    return _DEFAULT_URL


class Command(BaseCommand):
    help = "Hace polling al /health del MCP y registra un mcp_health_checks."

    def add_arguments(self, parser):
        parser.add_argument(
            "--url",
            default=None,
            help="URL del endpoint /health (override de settings/env).",
        )
        parser.add_argument(
            "--timeout",
            type=float,
            default=_DEFAULT_TIMEOUT,
            help=f"Timeout en segundos (default {_DEFAULT_TIMEOUT}).",
        )

    def handle(self, *args, **options):
        url = _resolve_url(options.get("url"))
        timeout = options.get("timeout") or _DEFAULT_TIMEOUT

        status, latency_ms, http_status, error = self._probe(url, timeout)

        McpHealthCheck.objects.create(
            checked_at=_now(),
            status=status,
            latency_ms=latency_ms,
            http_status=http_status,
            error=(error or None),
        )

        line = f"[poll_mcp_health] {status} url={url} http={http_status} latency_ms={latency_ms}"
        if status == McpHealthCheck.STATUS_UP:
            self.stdout.write(self.style.SUCCESS(line))
        elif status == McpHealthCheck.STATUS_DEGRADED:
            self.stdout.write(self.style.WARNING(line + f" error={error}"))
        else:
            self.stderr.write(line + f" error={error}")

    def _probe(self, url: str, timeout: float) -> tuple[str, int | None, int | None, str]:
        """Devuelve (status, latency_ms, http_status, error)."""
        req = urllib.request.Request(url, method="GET")
        started = time.monotonic()
        try:
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                latency_ms = int((time.monotonic() - started) * 1000)
                http_status = resp.getcode()
                body = resp.read(4096).decode("utf-8", errors="replace")
                return self._classify(http_status, body, latency_ms)
        except urllib.error.HTTPError as exc:
            # El server respondio pero con codigo de error (>=400).
            latency_ms = int((time.monotonic() - started) * 1000)
            return (
                McpHealthCheck.STATUS_DOWN,
                latency_ms,
                exc.code,
                f"HTTPError {exc.code}: {exc.reason}",
            )
        except (urllib.error.URLError, TimeoutError, OSError) as exc:
            # Timeout / conexion rechazada / DNS: el MCP esta caido o inalcanzable.
            latency_ms = int((time.monotonic() - started) * 1000)
            return (McpHealthCheck.STATUS_DOWN, latency_ms, None, f"{type(exc).__name__}: {exc}")
        except Exception as exc:  # noqa: BLE001 - el poller nunca debe romper el cron.
            latency_ms = int((time.monotonic() - started) * 1000)
            return (McpHealthCheck.STATUS_DOWN, latency_ms, None, f"{type(exc).__name__}: {exc}")

    @staticmethod
    def _classify(http_status: int, body: str, latency_ms: int) -> tuple[str, int, int, str]:
        if http_status != 200:
            # 2xx/3xx no-200: responde pero no del todo OK.
            return (McpHealthCheck.STATUS_DEGRADED, latency_ms, http_status, "")
        # 200: intentar leer el campo status del body JSON.
        try:
            payload = json.loads(body)
        except (ValueError, TypeError):
            # 200 sin JSON parseable: lo damos por arriba igual.
            return (McpHealthCheck.STATUS_UP, latency_ms, http_status, "")
        reported = str(payload.get("status", "")).lower()
        if reported in ("ok", "up", "healthy", ""):
            return (McpHealthCheck.STATUS_UP, latency_ms, http_status, "")
        return (
            McpHealthCheck.STATUS_DEGRADED,
            latency_ms,
            http_status,
            f"status reportado: {reported}",
        )


def _now():
    from django.utils import timezone

    return timezone.now()
