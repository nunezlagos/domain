"""Forms del mantenedor de Proyectos (migrado a core).

ProjectForm reusa core.forms.InstanceAwareMixin para capturar `instance=` y
poder excluirse a sí mismo en la validación de unicidad del slug en edición
(mismo helper `_exclude_self` que usa el resto de los mantenedores).

NO se reusa SlugNormalizationMixin: ese mixin NORMALIZA con slugify (transforma
"con espacios" -> "con-espacios"), pero el dominio de proyectos exige RECHAZAR
slugs mal formados (espacios/símbolos), no corregirlos. Por eso clean_slug
mantiene la validación estricta por regex.

Se usa forms.Form (no ModelForm) porque el modelo es managed=False.

Los repositorios git NO son fields del form: se manejan como filas dinámicas
(url + rama + folder) que viajan como arrays paralelos en el POST
(repo_url[], repo_branch[], repo_folder[]) y los parsea la view. La URL
principal del proyecto (projects.repository_url) se deriva del primer repo.
El campo `template` se quitó: su lógica nunca se consumía.
"""
import re

from django import forms

from core.forms import InstanceAwareMixin

from .models import Project

SLUG_RE = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")


class ProjectForm(InstanceAwareMixin, forms.Form):
    """Form para crear/editar proyectos (datos base; los repos van aparte)."""

    name = forms.CharField(
        label="Nombre",
        max_length=255,
        widget=forms.TextInput(attrs={"class": "form-control"}),
    )
    slug = forms.CharField(
        label="Slug",
        max_length=100,
        widget=forms.TextInput(attrs={"class": "form-control", "autocomplete": "off"}),
        help_text="Identificador único global (minúsculas, guiones).",
    )
    description = forms.CharField(
        label="Descripción",
        required=False,
        widget=forms.Textarea(attrs={"class": "form-control", "rows": 3}),
    )

    def __init__(self, *args, instance: Project | None = None, **kwargs):
        # InstanceAwareMixin captura instance y lo expone como self.instance.
        super().__init__(*args, instance=instance, **kwargs)

        if instance is not None and not self.is_bound:
            self.fields["name"].initial = instance.name
            self.fields["slug"].initial = instance.slug
            self.fields["description"].initial = instance.description

    def clean_slug(self):
        slug = self.cleaned_data["slug"].strip().lower()
        if not SLUG_RE.match(slug):
            raise forms.ValidationError(
                "El slug solo admite minúsculas, números y guiones (sin espacios)."
            )
        qs = Project.objects.filter(slug=slug)
        if self._exclude_self(qs).exists():
            raise forms.ValidationError("Ya existe un proyecto con ese slug.")
        return slug


class ProjectSearchForm(forms.Form):
    """Búsqueda simple en el listado."""

    q = forms.CharField(
        label="Buscar",
        required=False,
        widget=forms.TextInput(attrs={
            "class": "form-control",
            "placeholder": "Nombre, slug, descripción o repo...",
        }),
    )
