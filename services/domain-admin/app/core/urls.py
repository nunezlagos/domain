"""Helper de routing estándar para un mantenedor.

`maintainer_urlpatterns(views, id_kwarg)` arma las 7 rutas estándar a partir de
una instancia de core.views.MaintainerViews (o cualquier objeto con métodos
list/signal/create/detail/edit/delete/toggle):

    ""                  -> list      (GET)
    "signal/"           -> signal    (GET, JSON)
    "nuevo/"            -> create    (GET/POST)
    "<uuid:pk>/"        -> detail    (GET)
    "<uuid:pk>/editar/" -> edit      (GET/POST)
    "<uuid:pk>/eliminar/" -> delete  (POST)
    "<uuid:pk>/toggle/" -> toggle    (POST)

El `id_kwarg` controla el nombre del kwarg de la URL (por defecto "pk"); las
vistas de MaintainerViews leen `kwargs[id_kwarg]`. Mantené el mismo id_kwarg
acá y en el MaintainerViews del app.

Uso en el urls.py del app::

    from core.urls import maintainer_urlpatterns
    from .views import views   # instancia de MaintainerViews

    app_name = "projects"
    urlpatterns = maintainer_urlpatterns(views)
"""
from __future__ import annotations

from django.urls import path
from django.views.decorators.http import require_http_methods


def maintainer_urlpatterns(views, id_kwarg: str = "pk") -> list:
    """Devuelve la lista de urlpatterns estándar para `views`.

    Aplica las restricciones de método de cada ruta (create/edit GET+POST,
    delete/toggle POST) igual que el app de referencia.
    """
    create = require_http_methods(["GET", "POST"])(views.create)
    edit = require_http_methods(["GET", "POST"])(views.edit)
    delete = require_http_methods(["POST"])(views.delete)
    toggle = require_http_methods(["POST"])(views.toggle)

    return [
        path("", views.list, name="list"),
        path("signal/", views.signal, name="signal"),
        path("nuevo/", create, name="create"),
        path(f"<uuid:{id_kwarg}>/", views.detail, name="detail"),
        path(f"<uuid:{id_kwarg}>/editar/", edit, name="edit"),
        path(f"<uuid:{id_kwarg}>/eliminar/", delete, name="delete"),
        path(f"<uuid:{id_kwarg}>/toggle/", toggle, name="toggle"),
    ]
