"""HU-52.3: context processor para el badge del sidebar.

Expone `skill_suggestions_pending` en TODOS los templates (el sidebar se
renderiza en cada pagina via base.html). Es un COUNT read-only barato sobre
`skill_suggestions`. Defensivo: si la tabla aun no existe (mig 000182 no
corrida) o la DB falla, devuelve 0 — el sidebar nunca debe romper el render.
"""
from __future__ import annotations

import logging

log = logging.getLogger(__name__)


def pending_badge(request) -> dict:
    # Solo cuesta una query cuando hay sesion (sidebar no se ve sin login).
    if not getattr(request, "session", None) or not request.session.get("authenticated"):
        return {"skill_suggestions_pending": 0}
    try:
        from .services import count_pending

        return {"skill_suggestions_pending": count_pending()}
    except Exception as exc:  # noqa: BLE001 - el badge nunca rompe el render.
        log.debug("skill_suggestions badge no disponible: %s", exc)
        return {"skill_suggestions_pending": 0}
