"""Forms del mantenedor de Flows (migrados a core).

FlowForm reusa core.forms.SlugNormalizationMixin para la normalizacion
strip+lower+slugify + unicidad del slug (excluyendo la propia instancia en
edicion). La logica propia —spec JSONB editado como texto JSON y los flags
booleanos— queda aqui.
"""
from django import forms

from core.forms import SlugNormalizationMixin

from .models import Flow


class FlowForm(SlugNormalizationMixin, forms.Form):
    """Form para crear/editar flows. Usa forms.Form (model es managed=False).

    `spec` es un JSONB obligatorio en la BD: se edita como texto JSON
    (forms.JSONField parsea y valida que sea JSON bien formado).
    """


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
        help_text="Identificador unico (minusculas, guiones).",
    )
    description = forms.CharField(
        label="Descripcion",
        required=False,
        widget=forms.Textarea(attrs={"class": "form-control", "rows": 2}),
    )
    spec = forms.JSONField(
        label="Spec (JSON)",
        required=False,  # {} es valido; Django trata {} como "empty", asi que


        initial=dict,
        widget=forms.Textarea(attrs={
            "class": "form-control form-control--code",
            "rows": 14,
            "spellcheck": "false",
        }),
        help_text="Definicion declarativa del DAG en JSON (objeto). Vacio = {}.",
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
        label="Version de seed",
        required=False,
        min_value=0,
        widget=forms.NumberInput(attrs={"class": "form-control"}),
    )

    def __init__(self, *args, instance: Flow | None = None, **kwargs):


        super().__init__(*args, instance=instance, **kwargs)

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



        spec = self.cleaned_data.get("spec")
        if spec in (None, "", [], (), {}):
            return {}
        if not isinstance(spec, dict):
            raise forms.ValidationError("El spec debe ser un objeto JSON.")
        return spec


class FlowSearchForm(forms.Form):
    """Busqueda simple en el listado."""

    q = forms.CharField(
        label="Buscar",
        required=False,
        widget=forms.TextInput(attrs={
            "class": "form-control",
            "placeholder": "Nombre, slug o descripcion...",
        }),
    )
