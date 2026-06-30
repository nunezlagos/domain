"""Service base generico para mantenedores.

`MaintainerService` extrae las dos operaciones de lectura comunes a TODOS los
mantenedores, que hoy estan copiadas (con otro nombre de clave) en cada app:

- `list(...)`  -> listado con busqueda + paginacion (equivalente a
  users.services.list_users, pero la clave de la lista es `items` en vez de
  `users` para que sea generico).
- `list_signal(model)` -> señal barata de cambios {count, version} para el
  refresh on-change (equivalente a users.services.get_list_signal).

Es generico por model: no asume nada del dominio salvo que el model tenga
`updated_at` (lo cumplen todos por heredar de core.models.BaseModel).

Cada app puede:
    1. Instanciar/derivar este service para list + signal, y
    2. Mantener sus operaciones de negocio propias (create/update/delete/
       toggle, validaciones) en su `services.py`, como hace users.
"""
from __future__ import annotations

from typing import Iterable

from django.db.models import Count, Max, Q


class MaintainerService:
    """Base reusable para list + signal de un mantenedor.

    Subclase minima::

        class ProjectService(MaintainerService):
            model = Project
            search_fields = ("name", "slug")

        data = ProjectService().list(search="foo", page=1, per_page=20)
        sig = ProjectService().list_signal()

    O bien usarlo sin subclasear pasando el model en cada llamada (los
    metodos aceptan overrides explicitos).
    """


    model = None

    search_fields: tuple[str, ...] = ()

    ordering: tuple[str, ...] = ("-created_at",)

    def _resolve_model(self, model=None):
        m = model or self.model
        if m is None:
            raise ValueError("MaintainerService requiere un `model` (atributo o argumento).")
        return m

    def list(
        self,
        qs=None,
        search: str = "",
        search_fields: Iterable[str] | None = None,
        page: int = 1,
        per_page: int = 20,
    ) -> dict:
        """Listado con busqueda + paginacion.

        Retorna dict con: items, total, page, per_page, total_pages,
        has_next, has_prev. (Mismo shape que users.services.list_users salvo
        que la lista se llama `items`.)

        - `qs`: queryset base; si None usa `model.objects.all()`.
        - `search`: termino; filtra icontains en `search_fields` (OR), distinct.
        - `search_fields`: override de los campos de busqueda.
        """
        if qs is None:
            qs = self._resolve_model().objects.all()

        fields = tuple(search_fields) if search_fields is not None else self.search_fields
        if search and fields:
            cond = Q()
            for f in fields:
                cond |= Q(**{f"{f}__icontains": search})
            qs = qs.filter(cond)

        qs = qs.distinct().order_by(*self.ordering)

        total = qs.count()
        total_pages = max(1, (total + per_page - 1) // per_page)
        start = (page - 1) * per_page
        end = start + per_page
        items = list(qs[start:end])

        return {
            "items": items,
            "total": total,
            "page": page,
            "per_page": per_page,
            "total_pages": total_pages,
            "has_next": end < total,
            "has_prev": page > 1,
        }

    def list_signal(self, model=None) -> dict:
        """Señal barata de cambios para refresh on-change.

        Devuelve {count, version}: count = nº de filas, version =
        max(updated_at).isoformat() ("" si no hay filas). Cualquier alta,
        edicion, baja (soft) o toggle muta uno de los dos. El front compara
        contra su ultima señal y solo re-renderiza la tabla cuando algo cambio
        en la BD (incluyendo inserts de otros servicios).

        Query barata: SELECT count(*), max(updated_at) FROM <tabla>.
        """
        m = self._resolve_model(model)
        agg = m.objects.aggregate(total=Count("id"), latest=Max("updated_at"))
        latest = agg["latest"]
        return {
            "count": agg["total"] or 0,
            "version": latest.isoformat() if latest else "",
        }
