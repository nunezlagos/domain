"""Helpers base para factories de los apps.

Los PK son UUID (los genera domain-mcp en prod), así que en tests conviene
pasarlos explícitos. Estos helpers centralizan eso para que las factories de
cada app no repitan la generación de uuid.
"""
from __future__ import annotations

import uuid


def new_id() -> uuid.UUID:
    """UUID nuevo para usar como PK en factories de test."""
    return uuid.uuid4()


def make(model, /, **kwargs):
    """Crea una fila del `model` poniendo un PK uuid si no se pasó `id`.

    `model` es positional-only (con `/`) a propósito: hay tablas con una
    columna llamada `model` (ej. agents.model = nombre del LLM), así que
    `make(Agent, model="anthropic")` debe meter `model` en kwargs, no chocar
    con el parámetro de la clase.

    Ej.: make(Project, name="X", slug="x")
    """
    # Solo inyectar id si el modelo TIENE columna id (ej. UserRole no la tiene:
    # PK compuesta user_id+role_id).
    if "id" not in kwargs and any(f.name == "id" for f in model._meta.fields):
        kwargs["id"] = new_id()
    return model.objects.create(**kwargs)
