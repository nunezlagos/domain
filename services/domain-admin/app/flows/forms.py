"""Forms del mantenedor de Flows."""
from django import forms

from .models import Flow


class FlowForm(forms.Form):
    """Form para crear/editar flows.

    Usa forms.Form (no ModelForm) porque el modelo es managed=False.
    El slug es único; en edición se excluye el propio registro de la
    validación de unicidad.

    `spec` es un JSONB obligatorio en la BD: se edita como texto JSON
    (forms.JSONField parsea y valida que sea JSON bien formado).
    """

    name = forms.CharField(
        label="Nombre",
        max_length=255,
        widget=forms.TextInput(attrs={"class": "form-control"}),
    )
    slug = forms.SlugField(
        label="Slug",
        max_length=100,
        widget=forms.TextInput(attrs={"class": "form-control", "autocomplete": "off"}),
        help_text="Identificador único (minúsculas, guiones).",
    )
    description = forms.CharField(
        label="Descripción",
        required=False,
        widget=forms.Textarea(attrs={"class": "form-control", "rows": 2}),
    )
    spec = forms.JSONField(
        label="Spec (JSON)",
        required=False,  # {} es válido; Django trata {} como "empty", así que
                         # no lo marcamos required y normalizamos None/empty a {}
                         # en clean_spec (la columna es NOT NULL pero {} la satisface).
        initial=dict,
        widget=forms.Textarea(attrs={
            "class": "form-control",
            "rows": 6,
            "spellcheck": "false",
            "style": "font-family: var(--font-mono, monospace);",
        }),
        help_text="Definición declarativa del DAG en JSON (objeto). Vacío = {}.",
    )
    is_active = forms.BooleanField(
        label="Activo",
        required=False,
        initial=True,
        widget=forms.CheckboxInput(attrs={"class": "form-check-input"}),
    )
    deterministic_replay = forms.BooleanField(
        label="Replay determinista",
        required=False,
        widget=forms.CheckboxInput(attrs={"class": "form-check-input"}),
    )
    seed_managed = forms.BooleanField(
        label="Seed gestionado",
        required=False,
        widget=forms.CheckboxInput(attrs={"class": "form-check-input"}),
    )
    seed_version = forms.IntegerField(
        label="Versión de seed",
        required=False,
        min_value=0,
        widget=forms.NumberInput(attrs={"class": "form-control"}),
    )

    def __init__(self, *args, instance: Flow | None = None, **kwargs):
        super().__init__(*args, **kwargs)
        # clean_slug() consulta self.instance para excluirse en edición.
        self.instance = instance
        if instance is not None:
            # Valores iniciales solo al renderizar (unbound).
            if not self.is_bound:
                self.fields["name"].initial = instance.name
                self.fields["slug"].initial = instance.slug
                self.fields["description"].initial = instance.description
                self.fields["spec"].initial = instance.spec
                self.fields["is_active"].initial = instance.is_active
                self.fields["deterministic_replay"].initial = instance.deterministic_replay
                self.fields["seed_managed"].initial = instance.seed_managed
                self.fields["seed_version"].initial = instance.seed_version

    def clean_spec(self):
        # forms.JSONField ya parseó/validó el JSON (un string mal formado falla
        # antes de llegar acá). Solo normalizamos vacío -> {} (la columna jsonb
        # es NOT NULL) y exigimos que sea un objeto JSON.
        spec = self.cleaned_data.get("spec")
        if spec in (None, "", [], (), {}):
            return {}
        if not isinstance(spec, dict):
            raise forms.ValidationError("El spec debe ser un objeto JSON.")
        return spec

    def clean_slug(self):
        slug = self.cleaned_data["slug"].strip().lower()
        # Unicidad global del slug; en edición se excluye el propio registro.
        qs = Flow.objects.filter(slug=slug)
        if self.instance is not None:
            qs = qs.exclude(pk=self.instance.pk)
        if qs.exists():
            raise forms.ValidationError("Ya existe un flow con ese slug.")
        return slug


class FlowSearchForm(forms.Form):
    """Búsqueda simple en el listado."""

    q = forms.CharField(
        label="Buscar",
        required=False,
        widget=forms.TextInput(attrs={
            "class": "form-control",
            "placeholder": "Nombre, slug o descripción...",
        }),
    )
