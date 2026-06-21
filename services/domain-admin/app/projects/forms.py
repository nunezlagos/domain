"""Forms del mantenedor de Proyectos.

Se usa forms.Form (no ModelForm) porque el modelo es managed=False.
El slug es único globalmente; clean_slug excluye el propio pk en edición.
"""
import re

from django import forms

from .models import Project, ProjectTemplate

SLUG_RE = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")


class ProjectForm(forms.Form):
    """Form para crear/editar proyectos."""

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
    repository_url = forms.CharField(
        label="Repositorio (URL principal)",
        max_length=500,
        required=False,
        widget=forms.URLInput(attrs={"class": "form-control", "placeholder": "https://github.com/org/repo"}),
    )
    template = forms.ChoiceField(
        label="Template",
        required=False,
        choices=[],  # se completa en __init__
        widget=forms.Select(attrs={"class": "form-control form-select"}),
        help_text="Template base preconfigurado (opcional).",
    )
    current_branch = forms.CharField(
        label="Rama actual",
        max_length=120,
        required=False,
        widget=forms.TextInput(attrs={"class": "form-control", "placeholder": "main"}),
    )

    def __init__(self, *args, instance: Project | None = None, **kwargs):
        super().__init__(*args, **kwargs)
        # clean_slug() consulta self.instance para excluirse en edición.
        self.instance = instance

        # Choices de templates: vacío + todos los disponibles.
        self.fields["template"].choices = [("", "— Sin template —")] + [
            (str(t.pk), f"{t.name} ({t.slug})")
            for t in ProjectTemplate.objects.all().order_by("slug")
        ]

        if instance is not None and not self.is_bound:
            self.fields["name"].initial = instance.name
            self.fields["slug"].initial = instance.slug
            self.fields["description"].initial = instance.description
            self.fields["repository_url"].initial = instance.repository_url
            self.fields["current_branch"].initial = instance.current_branch
            self.fields["template"].initial = str(instance.template_id) if instance.template_id else ""

    def clean_slug(self):
        slug = self.cleaned_data["slug"].strip().lower()
        if not SLUG_RE.match(slug):
            raise forms.ValidationError(
                "El slug solo admite minúsculas, números y guiones (sin espacios)."
            )
        qs = Project.objects.filter(slug=slug)
        if self.instance is not None:
            qs = qs.exclude(pk=self.instance.pk)
        if qs.exists():
            raise forms.ValidationError("Ya existe un proyecto con ese slug.")
        return slug

    def clean_template(self):
        """ChoiceField vacío -> None (no '')."""
        value = self.cleaned_data.get("template") or ""
        return value or None


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
