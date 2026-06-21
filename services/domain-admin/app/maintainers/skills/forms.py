"""Forms del mantenedor de Skills (migrados a core).

SkillForm reusa core.forms.InstanceAwareMixin para capturar la instancia en
edición (y poder excluirse en la validación de unicidad). NO usa
SlugNormalizationMixin porque la unicidad de slug es por scope
(project_id, slug), no global: ese check vive en clean_slug. El resto —parseo
de tags, choices de skill_type— queda acá.
"""
from django import forms

from core.forms import InstanceAwareMixin

from .models import Skill


class SkillForm(InstanceAwareMixin, forms.Form):
    """Form para crear/editar skills.

    Usa forms.Form (no ModelForm) porque el modelo es managed=False.
    El slug es único dentro de su scope (project_id); en edición se excluye el
    propio registro de la validación de unicidad. El scope (project_id) no se
    edita desde el admin.
    """

    slug = forms.SlugField(
        label="Slug",
        max_length=100,
        widget=forms.TextInput(attrs={"class": "form-control", "autocomplete": "off"}),
        help_text="Identificador único dentro del scope (minúsculas, guiones).",
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
        label="Descripción",
        required=False,
        widget=forms.Textarea(attrs={"class": "form-control", "rows": 2}),
    )
    content = forms.CharField(
        label="Contenido",
        required=False,
        widget=forms.Textarea(attrs={"class": "form-control", "rows": 4}),
        help_text="Cuerpo de la skill (prompt, código, etc.).",
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
        # InstanceAwareMixin captura instance -> self.instance (para clean_slug).
        super().__init__(*args, instance=instance, **kwargs)
        if instance is not None and not self.is_bound:
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
        # La unicidad real es (project_id, slug). En edición el scope no cambia,
        # así que validamos contra el project_id del instance; en alta el scope
        # es global (project_id NULL) desde el admin.
        project_id = self.instance.project_id if self.instance is not None else None
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
    """Búsqueda simple en el listado."""

    q = forms.CharField(
        label="Buscar",
        required=False,
        widget=forms.TextInput(attrs={
            "class": "form-control",
            "placeholder": "Nombre, slug o descripción...",
        }),
    )
