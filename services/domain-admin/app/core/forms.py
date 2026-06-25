"""Mixins reutilizables para forms de mantenedores.

Extraen normalizaciones que se repiten en los forms de los apps:

- `InstanceAwareMixin`: guarda `self.instance` desde el kwarg `instance=` para
  que los `clean_*` puedan excluirse a si mismos en validaciones de unicidad
  (mismo patron que UserForm en el app de referencia).
- `EmailNormalizationMixin`: `clean_email` -> strip + lower + unicidad
  (excluye la propia instancia en edicion).
- `SlugNormalizationMixin`: `clean_slug` -> strip + lower + slugify + unicidad.

Configura por atributos de clase que model/campo usar para la unicidad::

    class ProjectForm(SlugNormalizationMixin, forms.Form):
        slug_model = Project          # model para chequear unicidad
        slug_field = "slug"           # nombre de la columna (default "slug")
        slug = forms.SlugField(...)

Si `*_model` es None se omite el check de unicidad (solo normaliza).
"""
from __future__ import annotations

from django import forms
from django.utils.text import slugify


class InstanceAwareMixin:
    """Captura `instance=` y lo expone como `self.instance` (None si no se paso).

    Permite que los `clean_*` hagan `.exclude(pk=self.instance.pk)` en edicion.
    """

    def __init__(self, *args, instance=None, **kwargs):
        super().__init__(*args, **kwargs)

        if not hasattr(self, "instance") or instance is not None:
            self.instance = instance

    def _exclude_self(self, qs):
        instance = getattr(self, "instance", None)
        if instance is not None and getattr(instance, "pk", None):
            qs = qs.exclude(pk=instance.pk)
        return qs


class EmailNormalizationMixin(InstanceAwareMixin):
    """Normaliza `email` a strip+lower y valida unicidad.

    Atributos opcionales: `email_model`, `email_field` (default "email").
    """

    email_model = None
    email_field = "email"

    def clean_email(self):
        email = (self.cleaned_data["email"] or "").strip().lower()
        if self.email_model is not None:
            qs = self.email_model.objects.filter(**{self.email_field: email})
            if self._exclude_self(qs).exists():
                raise forms.ValidationError("Ya existe un registro con ese email.")
        return email


class SlugNormalizationMixin(InstanceAwareMixin):
    """Normaliza `slug` a strip+lower+slugify y valida unicidad.

    Atributos opcionales: `slug_model`, `slug_field` (default "slug").
    """

    slug_model = None
    slug_field = "slug"

    def clean_slug(self):
        raw = (self.cleaned_data["slug"] or "").strip().lower()
        slug = slugify(raw)
        if not slug:
            raise forms.ValidationError("El slug no puede quedar vacio.")
        if self.slug_model is not None:
            qs = self.slug_model.objects.filter(**{self.slug_field: slug})
            if self._exclude_self(qs).exists():
                raise forms.ValidationError("Ya existe un registro con ese slug.")
        return slug
