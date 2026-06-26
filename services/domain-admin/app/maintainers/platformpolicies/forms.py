"""Forms del mantenedor de Politicas de plataforma.

forms.Form (modelo managed=False). La unicidad practica es (slug, is_active)
entre las politicas activas no borradas.
"""
from __future__ import annotations

from django import forms

from core.forms import InstanceAwareMixin

from .models import PlatformPolicy


class PlatformPolicyForm(InstanceAwareMixin, forms.Form):
    """Form para crear/editar politicas de plataforma."""

    name = forms.CharField(
        label="Nombre",
        max_length=160,
        widget=forms.TextInput(attrs={"class": "form-control"}),
    )
    slug = forms.SlugField(
        label="Slug",
        max_length=80,
        widget=forms.TextInput(attrs={"class": "form-control", "autocomplete": "off"}),
        help_text="Identificador unico de la politica (minusculas, guiones).",
    )
    kind = forms.ChoiceField(
        label="Tipo",
        choices=PlatformPolicy.KIND_CHOICES,
        widget=forms.Select(attrs={"class": "form-control form-select"}),
    )
    body_md = forms.CharField(
        label="Cuerpo (markdown)",
        widget=forms.Textarea(attrs={"class": "form-control form-control--code", "rows": 12, "spellcheck": "false"}),
        help_text="Texto de la politica que el orquestador inyecta en el system prompt.",
    )
    is_active = forms.BooleanField(
        label="Activa",
        required=False,
        initial=True,
        widget=forms.CheckboxInput(attrs={"class": "form-check-input"}),
    )

    def __init__(self, *args, instance: PlatformPolicy | None = None, **kwargs):
        super().__init__(*args, instance=instance, **kwargs)
        if instance is not None:
            if not self.is_bound:
                self.fields["name"].initial = instance.name
                self.fields["slug"].initial = instance.slug
                self.fields["kind"].initial = instance.kind
                self.fields["body_md"].initial = instance.body_md
                self.fields["is_active"].initial = instance.is_active

    def clean_slug(self):
        return self.cleaned_data["slug"].strip().lower()

    def clean(self):
        cleaned = super().clean()
        slug = cleaned.get("slug")
        if not slug:
            return cleaned

        qs = PlatformPolicy.objects.filter(slug=slug, deleted_at__isnull=True)
        if self._exclude_self(qs).exists():
            raise forms.ValidationError(
                "Ya existe una politica con ese slug en plataforma."
            )
        return cleaned


class PlatformPolicySearchForm(forms.Form):
    q = forms.CharField(
        label="Buscar",
        required=False,
        widget=forms.TextInput(attrs={
            "class": "form-control",
            "placeholder": "Nombre, slug o cuerpo...",
        }),
    )
