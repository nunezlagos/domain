"""Helpers para crear filas reales en la DB de test.

Los PKs son UUID (en prod los genera domain-mcp), así que en tests hay
que pasarlos explícitamente. organization_id también es un uuid explícito.
"""
from __future__ import annotations

import uuid

from clients.models import Client

# Org por defecto compartida entre helpers, para que los slugs choquen
# (la unicidad real es por (organization_id, slug)).
DEFAULT_ORG = uuid.UUID("11111111-1111-1111-1111-111111111111")


def make_client(
    name: str,
    *,
    slug: str | None = None,
    organization_id: uuid.UUID | str = DEFAULT_ORG,
    tax_id: str = "",
    contact_email: str = "",
    contact_phone: str = "",
    address: str = "",
    status: str = "active",
    deleted: bool = False,
) -> Client:
    if slug is None:
        slug = name.lower().replace(" ", "-")
    c = Client.objects.create(
        id=uuid.uuid4(),
        organization_id=organization_id,
        name=name,
        slug=slug,
        tax_id=tax_id,
        contact_email=contact_email,
        contact_phone=contact_phone,
        address=address,
        status=status,
    )
    if deleted:
        from django.utils import timezone
        c.deleted_at = timezone.now()
        c.status = "archived"
        c.save()
    return c
