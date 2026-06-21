"""Forms del mantenedor de Prompts."""
from django import forms

from .models import Prompt


class PromptForm(forms.Form):
    """Form para crear/editar prompts.

    Usa forms.Form (no ModelForm) porque el modelo es managed=False.
    La unicidad real es (organization_id, project_id, slug, version); en
    edición se excluye el propio registro de la validación de unicidad.
    """

    organization_id = forms.UUIDField(
        label="Organización (UUID)",
        widget=forms.TextInput(attrs={"class": "form-control", "autocomplete": "off"}),
        help_text="UUID de la organización dueña de este prompt.",
    )
    project_id = forms.UUIDField(
        label="Proyecto (UUID)",
        required=False,
        widget=forms.TextInput(attrs={"class": "form-control", "autocomplete": "off"}),
        help_text="UUID del proyecto (opcional).",
    )
    slug = forms.SlugField(
        label="Slug",
        max_length=100,
        widget=forms.TextInput(attrs={"class": "form-control", "autocomplete": "off"}),
        help_text="Identificador del prompt dentro del contexto (minúsculas, guiones).",
    )
    version = forms.IntegerField(
        label="Versión",
        min_value=1,
        initial=1,
        widget=forms.NumberInput(attrs={"class": "form-control"}),
    )
    body = forms.CharField(
        label="Cuerpo del prompt",
        widget=forms.Textarea(attrs={"class": "form-control", "rows": 8}),
    )
    description = forms.CharField(
        label="Descripción",
        required=False,
        widget=forms.Textarea(attrs={"class": "form-control", "rows": 2}),
    )
    tags = forms.CharField(
        label="Tags",
        required=False,
        widget=forms.TextInput(attrs={"class": "form-control"}),
        help_text="Lista separada por comas (ej: soporte, ventas).",
    )
    is_active = forms.BooleanField(
        label="Activo",
        required=False,
        initial=True,
        widget=forms.CheckboxInput(attrs={"class": "form-check-input"}),
    )

    def __init__(self, *args, instance: Prompt | None = None, **kwargs):
        super().__init__(*args, **kwargs)
        # clean() consulta self.instance para excluirse en edición.
        self.instance = instance
        if instance is not None:
            # organization_id / project_id no se editan una vez creado.
            self.fields["organization_id"].required = False
            self.fields["organization_id"].widget.attrs["readonly"] = True
            self.fields["project_id"].widget.attrs["readonly"] = True
            # Valores iniciales solo al renderizar (unbound).
            if not self.is_bound:
                self.fields["organization_id"].initial = instance.organization_id
                self.fields["project_id"].initial = instance.project_id
                self.fields["slug"].initial = instance.slug
                self.fields["version"].initial = instance.version
                self.fields["body"].initial = instance.body
                self.fields["description"].initial = instance.description
                self.fields["tags"].initial = ", ".join(instance.tags or [])
                self.fields["is_active"].initial = instance.is_active

    def clean_slug(self):
        return self.cleaned_data["slug"].strip().lower()

    def clean_tags(self) -> list[str]:
        raw = self.cleaned_data.get("tags", "") or ""
        return [t.strip() for t in raw.split(",") if t.strip()]

    def clean(self):
        cleaned = super().clean()
        slug = cleaned.get("slug")
        version = cleaned.get("version")
        if not slug or version is None:
            return cleaned

        # En edición la org/proyecto no cambian; usamos las del instance. En
        # alta usamos el organization_id/project_id enviados en el form.
        if self.instance is not None:
            org_id = self.instance.organization_id
            project_id = self.instance.project_id
        else:
            org_id = cleaned.get("organization_id")
            project_id = cleaned.get("project_id")

        if org_id:
            qs = Prompt.objects.filter(
                organization_id=org_id,
                project_id=project_id,
                slug=slug,
                version=version,
            )
            if self.instance is not None:
                qs = qs.exclude(pk=self.instance.pk)
            if qs.exists():
                raise forms.ValidationError(
                    "Ya existe un prompt con ese slug y versión en este "
                    "contexto (organización/proyecto)."
                )
        return cleaned


class PromptSearchForm(forms.Form):
    """Búsqueda simple en el listado."""

    q = forms.CharField(
        label="Buscar",
        required=False,
        widget=forms.TextInput(attrs={
            "class": "form-control",
            "placeholder": "Slug, descripción o contenido...",
        }),
    )
