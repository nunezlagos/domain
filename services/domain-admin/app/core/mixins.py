from django.http import HttpResponseRedirect
from django.shortcuts import render

from .auth import is_ajax, require_auth


class AuthMixin:
    def _auth_check(self, request) -> HttpResponseRedirect | None:
        return require_auth(request)


class AjaxMixin:
    def _is_ajax(self, request) -> bool:
        return is_ajax(request)

    def _render_fragment(self, request, template: str, context: dict):
        return render(request, template, context)

    def _render_ajax_redirect(self, url: str):
        from django.http import HttpResponseRedirect
        return HttpResponseRedirect(url)


class ContextMixin:
    def list_context(self, data: dict, search: str) -> dict:
        return {
            self.list_key: data[self.list_key],
            "total": data["total"],
            "page": data["page"],
            "per_page": data["per_page"],
            "total_pages": data["total_pages"],
            "has_next": data["has_next"],
            "has_prev": data["has_prev"],
            "search": search,
        }

    def form_context(self, form, mode: str, instance, action: str) -> dict:
        return {"form": form, "mode": mode, "object": instance, "action": action}

    def detail_context(self, instance) -> dict:
        return {"object": instance}

    def build_form(self, *args, instance=None, **kwargs):
        return self.form_class(*args, instance=instance, **kwargs)
