"""Modelos base ABSTRACTOS para los mantenedores.

Las tablas reales viven en domain-mcp; en este admin los models son
`managed = False` (Django no las migra, solo lee/escribe via ORM). Estos
abstractos existen para que los models de los apps hereden los campos comunes
(id uuid, created_at, updated_at, y para SoftDeleteModel ademas deleted_at +
status) y NO los repitan a mano en cada app.

IMPORTANTE — contrato para las subclases:
    Como `abstract = True` NO se hereda al heredar Meta, cada subclase DEBE
    declarar su propio Meta con `db_table` y `managed = False`::

        class User(SoftDeleteModel):
            email = models.EmailField(unique=True)
            # ... campos propios ...

            class Meta:
                db_table = "users"
                managed = False
                ordering = ["-created_at"]

    Los campos id/created_at/updated_at (y deleted_at/status en
    SoftDeleteModel) ya vienen del abstracto: no los redeclares salvo que
    necesites choices/validaciones propias (p.ej. status con STATUS_CHOICES).
"""
from __future__ import annotations

import uuid

from django.db import models


class BaseModel(models.Model):
    """Base abstracta: PK uuid + timestamps automaticos.

    `created_at` es auto_now_add (se setea al insertar) y `updated_at` es
    auto_now (se actualiza en cada save). El bump de updated_at es lo que
    alimenta la señal de cambios (ver MaintainerService.list_signal).
    """

    id = models.UUIDField(primary_key=True, default=uuid.uuid4)
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)

    class Meta:
        abstract = True


class SoftDeleteModel(BaseModel):
    """Base abstracta con soft delete: agrega `deleted_at` + `status`.

    El soft delete NO borra la fila: marca `deleted_at` y suele setear
    `status = "revoked"` (eso lo hace el service del app, no el modelo).
    `status` se declara generico (CharField); una subclase puede redeclararlo
    con `choices` propias sin romper el esquema (misma columna)::

        status = models.CharField(max_length=20, default="active",
                                  choices=STATUS_CHOICES)
    """

    deleted_at = models.DateTimeField(null=True, blank=True)
    status = models.CharField(max_length=20, default="active")

    class Meta:
        abstract = True
