from __future__ import annotations

from django.db import models

from core.models import SoftDeleteModel


class Cron(SoftDeleteModel):


    TARGET_TYPE_CHOICES = [
        ("flow", "Flow"),
        ("agent", "Agent"),
        ("skill", "Skill"),
    ]

    created_by = models.UUIDField(null=True, blank=True)
    slug = models.CharField(max_length=100)
    name = models.CharField(max_length=255)
    description = models.TextField(blank=True, default="")
    cron_expression = models.CharField(max_length=100)
    timezone = models.CharField(max_length=50, default="UTC")
    target_type = models.CharField(
        max_length=20, default="flow", choices=TARGET_TYPE_CHOICES
    )
    target_id = models.UUIDField()
    inputs = models.JSONField(default=dict, blank=True)
    enabled = models.BooleanField(default=True)
    last_run_at = models.DateTimeField(null=True, blank=True)
    next_run_at = models.DateTimeField(null=True, blank=True)


    status = models.TextField(default="active")

    class Meta:
        db_table = "crons"
        managed = False
        ordering = ["-created_at"]

    def __str__(self) -> str:
        return f"{self.name} ({self.slug})"

    @property
    def display_name(self) -> str:
        return self.name or self.slug

    @property
    def is_active(self) -> bool:
        return self.enabled and self.deleted_at is None
