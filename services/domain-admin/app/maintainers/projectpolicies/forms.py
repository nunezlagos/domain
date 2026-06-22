"""Forms del mantenedor de Reglas por proyecto.

forms.Form (modelo managed=False). El proyecto se elige de un dropdown
(proyectos activos) en vez de pegar un UUID. La unicidad práctica es
(project_id, slug) entre las reglas activas no borradas.
"""
from django import forms

from core.forms import InstanceAwareMixin
from maintainers.projects.models import Project

from .models import ProjectPolicy


class ProjectPolicyForm(InstanceAwareMixin, forms.Form):
    """Form para crear/editar reglas de proyecto."""

    project = forms.ChoiceField(
        label="Proyecto",
        choices=[],  # se completa en __init__
        widget=forms.Select(attrs={"class": "form-control form-select"}),
        help_text="Proyecto al que aplica la regla.",
    )
    name = forms.CharField(
        label="Nombre",
        max_length=160,
        widget=forms.TextInput(attrs={"class": "form-control"}),
    )
    slug = forms.SlugField(
        label="Slug",
        max_length=80,
        widget=forms.TextInput(attrs={"class": "form-control", "autocomplete": "off"}),
        help_text="Identificador de la regla dentro del proyecto (minúsculas, guiones).",
    )
    kind = forms.ChoiceField(
        label="Tipo",
        choices=ProjectPolicy.KIND_CHOICES,
        widget=forms.Select(attrs={"class": "form-control form-select"}),
    )
    body_md = forms.CharField(
        label="Cuerpo (markdown)",
        widget=forms.Textarea(attrs={"class": "form-control form-control--code", "rows": 12, "spellcheck": "false"}),
        help_text="Texto de la regla que el orquestador inyecta en el system prompt.",
    )
    override_platform = forms.BooleanField(
        label="Reemplaza la regla de plataforma",
        required=False,
        widget=forms.CheckboxInput(attrs={"class": "form-check-input"}),
        help_text="Si está activo, reemplaza la regla de plataforma del mismo tipo; si no, la amplía.",
    )
    is_active = forms.BooleanField(
        label="Activa",
        required=False,
        initial=True,
        widget=forms.CheckboxInput(attrs={"class": "form-check-input"}),
    )

    def __init__(self, *args, instance: ProjectPolicy | None = None, **kwargs):
        super().__init__(*args, instance=instance, **kwargs)
        self.fields["project"].choices = [
            (str(p.pk), f"{p.name} ({p.slug})")
            for p in Project.objects.filter(deleted_at__isnull=True).order_by("name")
        ]
        if instance is not None:
            # El proyecto no se cambia una vez creada la regla.
            self.fields["project"].disabled = True
            if not self.is_bound:
                self.fields["project"].initial = str(instance.project_id)
                self.fields["name"].initial = instance.name
                self.fields["slug"].initial = instance.slug
                self.fields["kind"].initial = instance.kind
                self.fields["body_md"].initial = instance.body_md
                self.fields["override_platform"].initial = instance.override_platform
                self.fields["is_active"].initial = instance.is_active

    def clean_slug(self):
        return self.cleaned_data["slug"].strip().lower()

    def clean(self):
        cleaned = super().clean()
        slug = cleaned.get("slug")
        if not slug:
            return cleaned
        # Proyecto: en edición se preserva el del instance (disabled no envía valor).
        if self.instance is not None:
            project_id = self.instance.project_id
        else:
            project_id = cleaned.get("project")
        qs = ProjectPolicy.objects.filter(
            project_id=project_id, slug=slug, deleted_at__isnull=True
        )
        if self._exclude_self(qs).exists():
            raise forms.ValidationError(
                "Ya existe una regla con ese slug en este proyecto."
            )
        return cleaned


class ProjectPolicySearchForm(forms.Form):
    q = forms.CharField(
        label="Buscar",
        required=False,
        widget=forms.TextInput(attrs={
            "class": "form-control",
            "placeholder": "Nombre, slug o cuerpo...",
        }),
    )
