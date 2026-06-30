from __future__ import annotations

from django.contrib import messages
from django.http import HttpResponseRedirect, JsonResponse
from django.shortcuts import render
from django.urls import reverse

from .auth import require_auth
from .mixins import AjaxMixin, AuthMixin, ContextMixin
from .service import MaintainerService


class MaintainerViews(AuthMixin, AjaxMixin, ContextMixin):
    def __init__(
        self,
        *,
        app_name: str,
        model,
        form_class,
        service,
        templates: str,
        search_fields=(),
        entity_label: str = "Registro",
        id_kwarg: str = "pk",
        list_key: str = "items",
        per_page: int = 20,
        search_param: str = "q",
    ):
        self.app_name = app_name
        self.model = model
        self.form_class = form_class
        self.service = service
        self.templates = templates.rstrip("/")
        self.search_fields = tuple(search_fields)
        self.entity_label = entity_label
        self.id_kwarg = id_kwarg
        self.list_key = list_key
        self.per_page = per_page
        self.search_param = search_param

        self._base = MaintainerService()
        self._base.model = model
        self._base.search_fields = self.search_fields

    def url(self, name: str, *args) -> str:
        return reverse(f"{self.app_name}:{name}", args=args)

    def tpl(self, name: str) -> str:
        return f"{self.templates}/{name}"

    @property
    def error_class(self):
        return getattr(self.service, "ServiceError", None) or getattr(
            self.service, f"{self.entity_label}Error", Exception
        )

    def do_list(self, search: str, page: int) -> dict:
        data = self._base.list(
            search=search, search_fields=self.search_fields,
            page=page, per_page=self.per_page,
        )
        if self.list_key != "items":
            data[self.list_key] = data.pop("items")
        return data

    def do_signal(self) -> dict:
        fn = getattr(self.service, "get_list_signal", None)
        if callable(fn):
            return fn()
        return self._base.list_signal()

    def do_get(self, obj_id):
        fn = getattr(self.service, "get_object", None) or getattr(
            self.service, f"get_{self._entity_attr()}", None
        )
        if callable(fn):
            return fn(obj_id)
        return self.model.objects.get(pk=obj_id)

    def do_create(self, form):
        fn = getattr(self.service, f"create_{self._entity_attr()}", None)
        if callable(fn):
            return fn(**self._form_payload(form))
        return self.model.objects.create(**self._form_payload(form))

    def do_update(self, instance, form):
        fn = getattr(self.service, f"update_{self._entity_attr()}", None)
        if callable(fn):
            return fn(instance, **self._form_payload(form))
        for k, v in self._form_payload(form).items():
            setattr(instance, k, v)
        instance.save()
        return instance

    def do_delete(self, instance) -> None:
        fn = getattr(self.service, f"delete_{self._entity_attr()}", None)
        if callable(fn):
            fn(instance)
            return
        from django.utils import timezone
        instance.deleted_at = timezone.now()
        instance.status = "revoked"
        instance.save()

    def do_toggle(self, instance) -> str:
        fn = getattr(self.service, f"toggle_{self._entity_attr()}_status", None)
        if callable(fn):
            return fn(instance)
        instance.status = "suspended" if instance.status == "active" else "active"
        instance.save()
        return instance.status

    def _entity_attr(self) -> str:
        return self.entity_label.strip().lower().replace(" ", "_")

    def _form_payload(self, form) -> dict:
        return dict(form.cleaned_data)

    def list(self, request):
        if (redir := self._auth_check(request)):
            return redir

        search = (request.GET.get(self.search_param) or "").strip()
        page = int(request.GET.get("page", 1) or 1)
        data = self.do_list(search=search, page=page)
        ctx = self.list_context(data, search)

        if request.GET.get("fragment") == "table":
            return render(request, self.tpl("_table_partial.html"), ctx)

        sig = self.do_signal()
        ctx["signal_count"] = sig["count"]
        ctx["signal_version"] = sig["version"]
        return render(request, self.tpl("list.html"), ctx)

    def signal(self, request):
        if (redir := self._auth_check(request)):
            return redir
        return JsonResponse(self.do_signal())

    def detail(self, request, **kwargs):
        if (redir := self._auth_check(request)):
            return redir
        obj_id = kwargs[self.id_kwarg]
        try:
            instance = self.do_get(obj_id)
        except self.error_class as exc:
            messages.error(request, str(exc))
            return HttpResponseRedirect(self.url("list"))
        ctx = self.detail_context(instance)
        if request.GET.get("partial") == "1":
            return render(request, self.tpl("_detail_partial.html"), ctx)
        return render(request, self.tpl("detail.html"), ctx)

    def create(self, request):
        if (redir := self._auth_check(request)):
            return redir

        if request.method == "POST":
            form = self.build_form(request.POST)
            if form.is_valid():
                try:
                    instance = self.do_create(form)
                    messages.success(
                        request, f"{self.entity_label} creado correctamente."
                    )
                    if self._is_ajax(request):
                        return self._render_ajax_redirect(self.url("list"))
                    return HttpResponseRedirect(self.url("detail", instance.pk))
                except self.error_class as exc:
                    messages.error(request, str(exc))
                    if self._is_ajax(request):
                        return render(
                            request, self.tpl("_form_partial.html"),
                            self.form_context(form, "create", None, self.url("create")),
                        )
        else:
            form = self.build_form()

        ctx = self.form_context(form, "create", None, self.url("create"))
        if request.GET.get("partial") == "1":
            return render(request, self.tpl("_form_partial.html"), ctx)
        return render(request, self.tpl("form.html"), ctx)

    def edit(self, request, **kwargs):
        if (redir := self._auth_check(request)):
            return redir
        obj_id = kwargs[self.id_kwarg]
        try:
            instance = self.do_get(obj_id)
        except self.error_class as exc:
            messages.error(request, str(exc))
            return HttpResponseRedirect(self.url("list"))

        if request.method == "POST":
            form = self.build_form(request.POST, instance=instance)
            if form.is_valid():
                try:
                    instance = self.do_update(instance, form)
                    messages.success(request, f"{self.entity_label} actualizado.")
                    if self._is_ajax(request):
                        return self._render_ajax_redirect(self.url("list"))
                    return HttpResponseRedirect(self.url("detail", instance.pk))
                except self.error_class as exc:
                    messages.error(request, str(exc))
                    if self._is_ajax(request):
                        return render(
                            request, self.tpl("_form_partial.html"),
                            self.form_context(form, "edit", instance, self.url("edit", instance.pk)),
                        )
        else:
            form = self.build_form(instance=instance)

        ctx = self.form_context(form, "edit", instance, self.url("edit", instance.pk))
        if request.GET.get("partial") == "1":
            return render(request, self.tpl("_form_partial.html"), ctx)
        return render(request, self.tpl("form.html"), ctx)

    def delete(self, request, **kwargs):
        if (redir := self._auth_check(request)):
            return redir
        obj_id = kwargs[self.id_kwarg]
        try:
            instance = self.do_get(obj_id)
            self.do_delete(instance)
            messages.success(
                request, f"{self.entity_label} eliminado (soft delete)."
            )
        except self.error_class as exc:
            messages.error(request, str(exc))
        return HttpResponseRedirect(self.url("list"))

    def toggle(self, request, **kwargs):
        if (redir := self._auth_check(request)):
            return redir
        obj_id = kwargs[self.id_kwarg]
        try:
            instance = self.do_get(obj_id)
            self.do_toggle(instance)
            messages.success(request, f"{self.entity_label} actualizado.")
        except self.error_class as exc:
            messages.error(request, str(exc))
        return HttpResponseRedirect(self.url("list"))
