"""views (controllers) del mantenedor de Agentes.

================================ SDD ================================
HU: Como administrador de la organización quiero un mantenedor de
    Agentes para crear, listar, editar y dar de baja (soft) los agentes
    LLM (provider/model/system_prompt/skills/límites) que la organización
    define, y ver su historial de versiones y los templates disponibles.

Criterios de aceptación:
  1. El listado muestra los agentes NO eliminados, paginado, con búsqueda
     server-side por nombre/slug/proveedor/modelo.
  2. El listado se auto-refresca solo cuando la BD cambia (señal
     count + max(updated_at)), no con polling ciego.
  3. Crear/editar se hacen en un modal AJAX (?partial=1 + header
     X-Requested-With: fetch). Form inválido re-renderiza el modal con
     errores; submit OK recarga el listado.
  4. El detalle se abre en modal (?partial=1) y lista, READ-ONLY, el
     historial de versiones del agent (agent_versions) y el catálogo de
     templates de agente (agent_templates). Ninguno de los dos tiene CRUD.
  5. Eliminar es soft-delete (POST): marca deleted_at, NO borra la fila.
     NO hay toggle de estado: la tabla agents no tiene columna status.
  6. Toda acción exige sesión autenticada; si no, redirige a /login/.
====================================================================

Cada view:
1. Auth guard.
2. Llama al service correspondiente (lógica de negocio).
3. Renderiza template (o redirige). Errores de dominio -> messages.error.
"""
from __future__ import annotations

from django.contrib import messages
from django.http import HttpResponseRedirect, JsonResponse
from django.shortcuts import render
from django.urls import reverse
from django.views.decorators.http import require_http_methods

from . import services
from .forms import AgentForm, AgentSearchForm


def _require_auth(request):
    """Redirect a /login/ si no está autenticado."""
    if not request.session.get("authenticated"):
        return HttpResponseRedirect("/login/")
    return None


def _is_ajax(request) -> bool:
    """El front manda el header literal 'fetch' (no 'XMLHttpRequest')."""
    return request.headers.get("X-Requested-With") == "fetch"


# === Listado ===

def agent_list(request):
    if (redir := _require_auth(request)):
        return redir

    search_form = AgentSearchForm(request.GET or None)
    search = ""
    if search_form.is_valid():
        search = search_form.cleaned_data.get("q", "").strip()

    page_num = int(request.GET.get("page", 1) or 1)
    per_page = 20

    data = services.list_agents(search=search, page=page_num, per_page=per_page)

    # ?fragment=table → solo tabla + paginación (sin base/layout), para
    # el refresh on-change y la paginación/búsqueda AJAX.
    if request.GET.get("fragment") == "table":
        return render(request, "agents/_table_partial.html", {
            "agents": data["agents"],
            "total": data["total"],
            "page": data["page"],
            "per_page": data["per_page"],
            "total_pages": data["total_pages"],
            "has_next": data["has_next"],
            "has_prev": data["has_prev"],
            "search": search,
        })

    # Señal inicial embebida: el front parte con la versión exacta del render.
    sig = services.get_list_signal()
    return render(request, "agents/list.html", {
        "agents": data["agents"],
        "total": data["total"],
        "page": data["page"],
        "per_page": data["per_page"],
        "total_pages": data["total_pages"],
        "has_next": data["has_next"],
        "has_prev": data["has_prev"],
        "search": search,
        "search_form": search_form,
        "signal_count": sig["count"],
        "signal_version": sig["version"],
    })


def agent_list_signal(request):
    """Señal de cambios (JSON) para refresh on-change. Query barata."""
    if (redir := _require_auth(request)):
        return redir
    return JsonResponse(services.get_list_signal())


# === Detalle ===

def agent_detail(request, agent_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        agent = services.get_agent(agent_id)
    except services.AgentError as exc:
        messages.error(request, str(exc))
        return HttpResponseRedirect(reverse("agents:list"))

    ctx = {
        "agent_obj": agent,
        "agent_versions": services.get_agent_versions(agent),
        "agent_templates": services.get_agent_templates(),
    }
    # ?partial=1 → solo el bloque detail (para modal).
    if request.GET.get("partial") == "1":
        return render(request, "agents/_detail_partial.html", ctx)
    return render(request, "agents/detail.html", ctx)


# === Crear ===

@require_http_methods(["GET", "POST"])
def agent_create(request):
    if (redir := _require_auth(request)):
        return redir

    if request.method == "POST":
        form = AgentForm(request.POST)
        if form.is_valid():
            try:
                agent = services.create_agent(
                    name=form.cleaned_data["name"],
                    slug=form.cleaned_data["slug"],
                    provider=form.cleaned_data["provider"],
                    model=form.cleaned_data["model"],
                    description=form.cleaned_data["description"],
                    system_prompt=form.cleaned_data["system_prompt"],
                    skills_slugs=form.cleaned_data["skills_slugs"],
                    max_iterations=form.cleaned_data["max_iterations"],
                    token_budget=form.cleaned_data["token_budget"],
                    temperature=form.cleaned_data["temperature"],
                )
                messages.success(request, f"Agente {agent.name} creado correctamente.")
                if _is_ajax(request):
                    return HttpResponseRedirect(reverse("agents:list"))
                return HttpResponseRedirect(reverse("agents:detail", args=[agent.pk]))
            except services.AgentError as exc:
                messages.error(request, str(exc))
                if _is_ajax(request):
                    return render(request, "agents/_form_partial.html", {
                        "form": form, "mode": "create", "agent_obj": None,
                        "action": reverse("agents:create"),
                    })
    else:
        form = AgentForm()

    ctx = {"form": form, "mode": "create", "agent_obj": None,
           "action": reverse("agents:create")}
    if request.GET.get("partial") == "1":
        return render(request, "agents/_form_partial.html", ctx)
    return render(request, "agents/form.html", ctx)


# === Editar ===

@require_http_methods(["GET", "POST"])
def agent_edit(request, agent_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        agent = services.get_agent(agent_id)
    except services.AgentError as exc:
        messages.error(request, str(exc))
        return HttpResponseRedirect(reverse("agents:list"))

    if request.method == "POST":
        form = AgentForm(request.POST, instance=agent)
        if form.is_valid():
            try:
                agent = services.update_agent(
                    agent,
                    name=form.cleaned_data["name"],
                    slug=form.cleaned_data["slug"],
                    provider=form.cleaned_data["provider"],
                    model=form.cleaned_data["model"],
                    description=form.cleaned_data["description"],
                    system_prompt=form.cleaned_data["system_prompt"],
                    skills_slugs=form.cleaned_data["skills_slugs"],
                    max_iterations=form.cleaned_data["max_iterations"],
                    token_budget=form.cleaned_data["token_budget"],
                    temperature=form.cleaned_data["temperature"],
                )
                messages.success(request, f"Agente {agent.name} actualizado.")
                if _is_ajax(request):
                    return HttpResponseRedirect(reverse("agents:list"))
                return HttpResponseRedirect(reverse("agents:detail", args=[agent.pk]))
            except services.AgentError as exc:
                messages.error(request, str(exc))
                if _is_ajax(request):
                    return render(request, "agents/_form_partial.html", {
                        "form": form, "mode": "edit", "agent_obj": agent,
                        "action": reverse("agents:edit", args=[agent.pk]),
                    })
    else:
        form = AgentForm(instance=agent)

    ctx = {"form": form, "mode": "edit", "agent_obj": agent,
           "action": reverse("agents:edit", args=[agent.pk])}
    if request.GET.get("partial") == "1":
        return render(request, "agents/_form_partial.html", ctx)
    return render(request, "agents/form.html", ctx)


# === Eliminar (soft) ===

@require_http_methods(["POST"])
def agent_delete(request, agent_id: str):
    if (redir := _require_auth(request)):
        return redir

    try:
        agent = services.get_agent(agent_id)
        services.delete_agent(agent)
        messages.success(request, f"Agente {agent.name} eliminado (soft delete).")
    except services.AgentError as exc:
        messages.error(request, str(exc))

    return HttpResponseRedirect(reverse("agents:list"))
