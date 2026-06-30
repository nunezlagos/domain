from __future__ import annotations

from django.db import models

from core.models import BaseModel, SoftDeleteModel


class Flow(SoftDeleteModel):
    slug = models.CharField(max_length=100)
    name = models.CharField(max_length=255)
    description = models.TextField(blank=True, default="")
    spec = models.JSONField(default=dict, blank=True)
    is_active = models.BooleanField(default=True)
    deterministic_replay = models.BooleanField(default=False)
    seed_managed = models.BooleanField(default=False)
    seed_version = models.IntegerField(null=True, blank=True)
    is_user_modified = models.BooleanField(default=False)

    class Meta:
        db_table = "flows"
        managed = False
        ordering = ["-created_at"]

    def __str__(self) -> str:
        return f"{self.name} ({self.slug})"

    @property
    def display_name(self) -> str:
        return self.name or self.slug

    @property
    def is_live(self) -> bool:
        return self.is_active and self.deleted_at is None

    @property
    def status_label(self) -> str:
        if self.deleted_at is not None:
            return "Eliminado"
        return "Activo" if self.is_active else "Inactivo"


class FlowVersion(BaseModel):
    flow = models.ForeignKey(
        Flow,
        on_delete=models.CASCADE,
        db_column="flow_id",
        related_name="versions",
    )
    version = models.IntegerField()
    definition = models.JSONField(default=dict, blank=True)
    hash = models.CharField(max_length=64)
    note = models.TextField(blank=True, default="")
    created_by = models.UUIDField(null=True, blank=True)
    status = models.CharField(max_length=20, default="active")
    is_default = models.BooleanField(default=False)
    published_at = models.DateTimeField(null=True, blank=True)
    deprecated_at = models.DateTimeField(null=True, blank=True)

    class Meta:
        db_table = "flow_versions"
        managed = False
        ordering = ["-version"]

    def __str__(self) -> str:
        return f"{self.flow_id} v{self.version}"

    @property
    def short_hash(self) -> str:
        return self.hash[:12] if self.hash else ""
