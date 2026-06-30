"""Forms del mantenedor de Skills (migrados a core).

SkillForm reusa core.forms.InstanceAwareMixin para capturar la instancia en
edicion (y poder excluirse en la validacion de unicidad). NO usa
SlugNormalizationMixin porque la unicidad de slug es por scope
(project_id, slug), no global: ese check vive en clean_slug. El resto —parseo
de tags, choices de skill_type— queda aqui.
"""
from django import forms

from core.forms import InstanceAwareMixin
from maintainers.projects.models import Project

from .models import Skill


class SkillForm(InstanceAwareMixin, forms.Form):
    """Form para crear/editar skills.

    Usa forms.Form (no ModelForm) porque el modelo es managed=False.
    El slug es unico dentro de su scope (project_id). El scope se elige al
    crear (Global o un proyecto); en edicion no cambia. root_path acota la
    skill a un subpath del repo (monorepo): vacio = todo el proyecto.
    """

    project = forms.ChoiceField(
        label="Scope",
        required=False,
        choices=[],  # se completa en __init__ (Global + proyectos)
        widget=forms.Select(attrs={"class": "form-control form-select"}),
        help_text="Global (toda la org) o un proyecto especifico.",
    )
    root_path = forms.CharField(
        label="Root path (monorepo)",
        required=False,
        max_length=255,
        widget=forms.TextInput(attrs={"class": "form-control", "autocomplete": "off"}),
        help_text="Subpath al que aplica en un monorepo (ej. services/api). Vacio = todo el proyecto.",
    )
    slug = forms.SlugField(
        label="Slug",
        max_length=100,
        widget=forms.TextInput(attrs={"class": "form-control", "autocomplete": "off"}),
        help_text="Identificador unico dentro del scope (minusculas, guiones).",
    )
    name = forms.CharField(
        label="Nombre",
        max_length=255,
        widget=forms.TextInput(attrs={"class": "form-control"}),
    )
    skill_type = forms.ChoiceField(
        label="Tipo",
        choices=Skill.SKILL_TYPE_CHOICES,
        initial="prompt",
        widget=forms.Select(attrs={"class": "form-control form-select"}),
    )
    description = forms.CharField(
        label="Descripcion",
        required=False,
        widget=forms.Textarea(attrs={"class": "form-control", "rows": 2}),
    )
    content = forms.CharField(
        label="Contenido",
        required=False,
        widget=forms.Textarea(attrs={"class": "form-control form-control--code", "rows": 14, "spellcheck": "false"}),
        help_text="Cuerpo de la skill (prompt, codigo, etc.) — editor monospace.",
    )
    timeout_seconds = forms.IntegerField(
        label="Timeout (segundos)",
        min_value=1,
        max_value=600,
        initial=30,
        widget=forms.NumberInput(attrs={"class": "form-control"}),
        help_text="Entre 1 y 600 segundos.",
    )
    tags = forms.CharField(
        label="Tags",
        required=False,
        widget=forms.TextInput(attrs={"class": "form-control"}),
        help_text="Separados por coma (ej. soporte, ventas).",
    )
    idempotent = forms.BooleanField(
        label="Idempotente",
        required=False,
        widget=forms.CheckboxInput(attrs={"class": "form-check-input"}),
    )
    has_side_effects = forms.BooleanField(
        label="Tiene efectos secundarios",
        required=False,
        widget=forms.CheckboxInput(attrs={"class": "form-check-input"}),
    )

    def __init__(self, *args, instance: Skill | None = None, **kwargs):

        super().__init__(*args, instance=instance, **kwargs)
        self.fields["project"].choices = [("", "— Global (toda la org) —")] + [
            (str(p.pk), f"{p.name} ({p.slug})")
            for p in Project.objects.filter(deleted_at__isnull=True).order_by("name")
        ]
        if instance is not None:
            # El scope no se cambia en edicion: se muestra fijo.
            self.fields["project"].disabled = True
            if not self.is_bound:
                self.fields["project"].initial = str(instance.project_id) if instance.project_id else ""
                self.fields["root_path"].initial = instance.root_path or ""
                self.fields["slug"].initial = instance.slug
                self.fields["name"].initial = instance.name
                self.fields["skill_type"].initial = instance.skill_type
                self.fields["description"].initial = instance.description
                self.fields["content"].initial = instance.content
                self.fields["timeout_seconds"].initial = instance.timeout_seconds
                self.fields["tags"].initial = ", ".join(instance.tags or [])
                self.fields["idempotent"].initial = instance.idempotent
                self.fields["has_side_effects"].initial = instance.has_side_effects

    def clean_slug(self):
        slug = self.cleaned_data["slug"].strip().lower()

        # En edicion el scope es el del registro; en alta, el elegido en el form.
        if self.instance is not None:
            project_id = self.instance.project_id
        else:
            project_id = self.data.get("project") or None
        qs = Skill.objects.filter(deleted_at__isnull=True, slug=slug)
        if project_id in (None, ""):
            qs = qs.filter(project_id__isnull=True)
        else:
            qs = qs.filter(project_id=project_id)
        if self._exclude_self(qs).exists():
            raise forms.ValidationError(
                "Ya existe una skill con ese slug en este scope."
            )
        return slug

    def clean_tags(self) -> list[str]:
        raw = self.cleaned_data.get("tags", "") or ""
        return [t.strip() for t in raw.split(",") if t.strip()]


class SkillSearchForm(forms.Form):
    """Busqueda simple en el listado."""

    q = forms.CharField(
        label="Buscar",
        required=False,
        widget=forms.TextInput(attrs={
            "class": "form-control",
            "placeholder": "Nombre, slug o descripcion...",
        }),
    )
