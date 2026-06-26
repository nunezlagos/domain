"""Modelo del monitoreo de uptime/health del server domain-mcp.

Tabla `mcp_health_checks` creada por la migracion Go 000174 (domain-mcp).
Es metrica de PLATAFORMA (no por-org): NO tiene organization_id ni RLS.

managed=False: Django NO la migra, solo lee/escribe via ORM. Las filas las
genera el management command `poll_mcp_health` (polling al /health del MCP).

NO hereda de core.BaseModel porque la tabla NO tiene `updated_at` (solo
`checked_at` + `created_at`). Cada columna declarada existe EXACTO en la
tabla real.
"""
from __future__ import annotations

import uuid

from django.db import models


class McpHealthCheck(models.Model):
    """Un chequeo de health del MCP en un instante dado."""

    STATUS_UP = "up"
    STATUS_DOWN = "down"
    STATUS_DEGRADED = "degraded"
    STATUS_CHOICES = [
        (STATUS_UP, "Up"),
        (STATUS_DOWN, "Down"),
        (STATUS_DEGRADED, "Degraded"),
    ]

    id = models.UUIDField(primary_key=True, default=uuid.uuid4)
    checked_at = models.DateTimeField()
    status = models.CharField(max_length=10, choices=STATUS_CHOICES)
    latency_ms = models.IntegerField(null=True, blank=True)
    http_status = models.IntegerField(null=True, blank=True)
    error = models.TextField(null=True, blank=True)
    created_at = models.DateTimeField(auto_now_add=True)

    class Meta:
        db_table = "mcp_health_checks"
        managed = False
        ordering = ["-checked_at"]

    def __str__(self) -> str:
        return f"{self.status} @ {self.checked_at:%Y-%m-%d %H:%M:%S}"
