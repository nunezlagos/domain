"""Forms del mantenedor de Agentes (migrados a core).

AgentForm reusa core.forms.SlugNormalizationMixin para la normalización
strip+lower+slugify + unicidad del slug (excluyendo la propia instancia en
edición). Lo propio del dominio —campos del agente y skills_slugs como CSV—
queda acá. Usa forms.Form (no ModelForm) porque el modelo es managed=False.
"""
from django import forms

from core.forms import SlugNormalizationMixin

from .models import Agent


class AgentForm(SlugNormalizationMixin, forms.Form):
    """Form para crear/editar agentes.

    El slug es único globalmente (ya no hay organización: organization_id fue
    dropeada). `skills_slugs` (text[] en la BD) se edita como CSV y se
    normaliza a lista en clean_skills_slugs().
    """

    # core.forms.SlugNormalizationMixin usa estos atributos para la unicidad.
    slug_model = Agent
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
    provider = forms.CharField(
        label="Proveedor",
        max_length=50,
        widget=forms.TextInput(attrs={"class": "form-control"}),
        help_text="Ej: anthropic, openai, google.",
    )
    model = forms.CharField(
        label="Modelo",
        max_length=100,
        widget=forms.TextInput(attrs={"class": "form-control"}),
        help_text="Ej: claude-haiku-4-5.",
    )
    description = forms.CharField(
        label="Descripción",
        required=False,
        widget=forms.Textarea(attrs={"class": "form-control", "rows": 2}),
    )
    system_prompt = forms.CharField(
        label="System prompt",
        required=False,
        widget=forms.Textarea(attrs={"class": "form-control", "rows": 4}),
    )
    skills_slugs = forms.CharField(
        label="Skills (slugs)",
        required=False,
        widget=forms.TextInput(attrs={"class": "form-control", "autocomplete": "off"}),
        help_text="Slugs de skills separados por coma.",
    )
    max_iterations = forms.IntegerField(
        label="Máx. iteraciones",
        min_value=1,
        initial=20,
        widget=forms.NumberInput(attrs={"class": "form-control"}),
    )
    token_budget = forms.IntegerField(
        label="Presupuesto de tokens",
        required=False,
        min_value=0,
        widget=forms.NumberInput(attrs={"class": "form-control"}),
        help_text="Opcional. Vacío = sin límite.",
    )
    temperature = forms.DecimalField(
        label="Temperatura",
        required=False,
        min_value=0,
        max_value=9.99,
        max_digits=3,
        decimal_places=2,
        widget=forms.NumberInput(attrs={"class": "form-control", "step": "0.01"}),
        help_text="Opcional. Ej: 0.70.",
    )

    def __init__(self, *args, instance: Agent | None = None, **kwargs):
        # InstanceAwareMixin (via SlugNormalizationMixin) captura instance para
        # que clean_slug se excluya a sí mismo en edición.
        super().__init__(*args, instance=instance, **kwargs)
        # Valores iniciales solo al renderizar el form de edición (unbound).
        if instance is not None and not self.is_bound:
            self.fields["name"].initial = instance.name
            self.fields["slug"].initial = instance.slug
            self.fields["provider"].initial = instance.provider
            self.fields["model"].initial = instance.model
            self.fields["description"].initial = instance.description
            self.fields["system_prompt"].initial = instance.system_prompt
            self.fields["skills_slugs"].initial = ", ".join(
                instance.skills_slugs or []
            )
            self.fields["max_iterations"].initial = instance.max_iterations
            self.fields["token_budget"].initial = instance.token_budget
            self.fields["temperature"].initial = instance.temperature

    def clean_skills_slugs(self):
        raw = self.cleaned_data.get("skills_slugs", "") or ""
        return [s.strip() for s in raw.split(",") if s.strip()]


class AgentSearchForm(forms.Form):
    """Búsqueda simple en el listado."""

    q = forms.CharField(
        label="Buscar",
        required=False,
        widget=forms.TextInput(attrs={
            "class": "form-control",
            "placeholder": "Nombre, slug, proveedor o modelo...",
        }),
    )
