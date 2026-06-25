"""Views del mantenedor de Flows (migradas a core).

================================ SDD ================================
HU: Como administrador de la organizacion quiero un mantenedor de Flows
    para crear, listar, editar, activar/desactivar y dar de baja (soft)
    los flows (DAGs declarativos), y ver el historial de versiones
    (snapshots inmutables) de cada flow.

Criterios de aceptacion:
  1. El listado muestra los flows NO eliminados, paginado, con busqueda
     server-side por nombre/slug/descripcion.
  2. El listado se auto-refresca solo cuando la BD cambia (señal
     count + max(updated_at)), no con polling ciego.
  3. Crear/editar se hacen en un modal AJAX (?partial=1 + header
     X-Requested-With: fetch). Form invalido re-renderiza el modal con
     errores; submit OK recarga el listado.
  4. El detalle se abre en modal (?partial=1) e incluye la lista
     READ-ONLY de versiones (flow_versions) del flow; sin CRUD sobre ellas.
  5. Toggle alterna is_active (habilitado <-> deshabilitado, POST).
     Eliminar es soft-delete (POST): marca deleted_at + is_active=false.
  6. Toda accion exige sesion autenticada; si no, redirige a /login/.
====================================================================

Las 7 vistas estandar (list, signal, detail, create, edit, delete, toggle) las
arma core.views.MaintainerViews. Aqui solo:

  1. Se configura la instancia `views` (model/form/service/templates/labels).
  2. Se sobreescriben los context builders especificos de flows:
       - form_context / detail_context: exponen `flow_obj` (+ flow_versions en
         detail) que los templates de flows ya consumen.

El payload del service (create_flow/update_flow) calza 1:1 con el cleaned_data
del FlowForm, asi que NO se sobreescribe _form_payload (el default
dict(form.cleaned_data) ya sirve).

El guard de auth (require_auth) y la deteccion AJAX (is_ajax) vienen de
core.auth (antes estaban duplicados como _require_auth/_is_ajax).
"""
from __future__ import annotations

import json

from django.contrib import messages
from django.http import HttpResponse, HttpResponseRedirect

from core.auth import require_auth
from core.views import MaintainerViews

from . import services
from .forms import FlowForm
from .models import Flow


def _pretty_spec(flow) -> str:
    """Devuelve el spec del flow como JSON formateado (indent 2) para mostrar/
    editar. Acepta dict/list (JSONField) o string JSON; si no parsea, lo crudo."""
    spec = getattr(flow, "spec", None)
    if spec in (None, ""):
        return ""
    if isinstance(spec, (dict, list)):
        return json.dumps(spec, indent=2, ensure_ascii=False)
    try:
        return json.dumps(json.loads(spec), indent=2, ensure_ascii=False)
    except (ValueError, TypeError):
        return str(spec)


class FlowViews(MaintainerViews):
    """MaintainerViews especializado para flows (list filtrado + context keys)."""



    def list(self, request):
        self._list_request = request
        return super().list(request)




    def create(self, request):
        if (redir := require_auth(request)):
            return redir
        messages.info(
            request,
            "Los flows no se crean desde el dashboard: el catalogo lo gestiona la plataforma.",
        )
        return HttpResponseRedirect(self.url("list"))





    def do_list(self, search: str, page: int) -> dict:
        req = getattr(self, "_list_request", None)
        val = req.GET.get("active") if req else None
        is_active = None if not val else (val == "1")
        return services.list_flows(
            search=search, page=page, per_page=self.per_page,
            is_active=is_active,
        )



    def list_context(self, data: dict, search: str) -> dict:
        ctx = super().list_context(data, search)
        req = getattr(self, "_list_request", None)
        ctx["selected_active"] = req.GET.get("active", "") if req else ""
        return ctx


    def form_context(self, form, mode: str, instance, action: str) -> dict:
        ctx = {
            "form": form,
            "mode": mode,
            "flow_obj": instance,
            "object": instance,
            "action": action,
        }

        if mode == "edit" and instance is not None:
            ctx["flow_versions"] = services.get_flow_versions(instance)
        return ctx

    def detail_context(self, instance) -> dict:
        return {
            "flow_obj": instance,
            "object": instance,
            "flow_versions": services.get_flow_versions(instance),
            "spec_pretty": _pretty_spec(instance),
        }




views = FlowViews(
    app_name="flows",
    model=Flow,
    form_class=FlowForm,
    service=services,
    templates="flows",
    search_fields=("name", "slug", "description"),
    entity_label="Flow",
    id_kwarg="flow_id",
    list_key="flows",
    per_page=10,
    search_param="q",
)


def export_flows(request):
    """Export CSV (consolidado, abre en Excel) de los flows filtrados.
    Respeta los filtros activos: q (busqueda) y active ("1"/"0", "" = todos)."""
    if (redir := require_auth(request)):
        return redir
    val = request.GET.get("active") or ""
    is_active = None if not val else (val == "1")
    csv_data = services.export_flows_csv(
        search=(request.GET.get("q") or "").strip(),
        is_active=is_active,
    )
    resp = HttpResponse(csv_data, content_type="text/csv; charset=utf-8")
    resp["Content-Disposition"] = 'attachment; filename="flows.csv"'
    return resp
