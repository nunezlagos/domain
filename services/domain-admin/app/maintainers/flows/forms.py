"""Forms del mantenedor de Flows (migrados a core).

FlowForm reusa core.forms.SlugNormalizationMixin para la normalización
strip+lower+slugify + unicidad del slug (excluyendo la propia instancia en
edición). La lógica propia —spec JSONB editado como texto JSON y los flags
booleanos— queda acá.
"""
from django import forms

from core.forms import SlugNormalizationMixin

from .models import Flow


class FlowForm(SlugNormalizationMixin, forms.Form):
    """Form para crear/editar flows. Usa forms.Form (model es managed=False).

    `spec` es un JSONB obligatorio en la BD: se edita como texto JSON
    (forms.JSONField parsea y valida que sea JSON bien formado).
    """

    # core.forms.SlugNormalizationMixin usa estos atributos para la unicidad.
    slug_model = Flow
    slug_field = "slug"

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
        # InstanceAwareMixin (via SlugNormalizationMixin) captura instance para
        # que clean_slug se excluya a sí mismo en edición.
        super().__init__(*args, instance=instance, **kwargs)
        # Valores iniciales solo al renderizar el form de edición (unbound).
        if instance is not None and not self.is_bound:
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
