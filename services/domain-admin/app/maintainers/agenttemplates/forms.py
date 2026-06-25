"""Forms del mantenedor de Plantillas de Agentes.

AgentTemplateForm usa core.forms.SlugNormalizationMixin: la unicidad de slug es
GLOBAL (la tabla agent_templates no tiene scope por proyecto), asi que el mixin
encaja 1:1 (normaliza strip+lower+slugify y valida unicidad excluyendo el propio
registro en edicion). El resto —choices, parseo de capabilities— vive aqui.

Usa forms.Form (no ModelForm) porque el modelo es managed=False (proxy).
"""
from django import forms

from core.forms import SlugNormalizationMixin

from .models import AgentTemplate


class AgentTemplateForm(SlugNormalizationMixin, forms.Form):
    """Form para crear/editar plantillas de agentes."""


    slug_model = AgentTemplate
    slug_field = "slug"

    slug = forms.SlugField(
        label="Slug",
        max_length=80,
        widget=forms.TextInput(attrs={"class": "form-control", "autocomplete": "off"}),
        help_text="Identificador unico global (minusculas, guiones).",
    )
    name = forms.CharField(
        label="Nombre",
        max_length=120,
        widget=forms.TextInput(attrs={"class": "form-control"}),
    )
    system_prompt = forms.CharField(
        label="System prompt",
        widget=forms.Textarea(attrs={"class": "form-control form-control--code", "rows": 14, "spellcheck": "false"}),
        help_text="Instrucciones de sistema de la plantilla (editor monospace).",
    )
    personality = forms.CharField(
        label="Personalidad",
        required=False,
        widget=forms.Textarea(attrs={"class": "form-control", "rows": 2}),
    )
    capabilities = forms.CharField(
        label="Capacidades",
        required=False,
        widget=forms.TextInput(attrs={"class": "form-control"}),
        help_text="Separadas por coma (ej. research, code).",
    )
    model = forms.CharField(
        label="Modelo",
        max_length=80,
        initial="claude-haiku-4-5",
        widget=forms.TextInput(attrs={"class": "form-control"}),
    )
    temperature = forms.DecimalField(
        label="Temperatura",
        max_digits=3,
        decimal_places=2,
        min_value=0,
        max_value=2,
        initial="0.7",
        widget=forms.NumberInput(attrs={"class": "form-control", "step": "0.01"}),
        help_text="Entre 0.00 y 2.00.",
    )
    max_tokens = forms.IntegerField(
        label="Max tokens",
        min_value=1,
        max_value=200000,
        initial=4096,
        widget=forms.NumberInput(attrs={"class": "form-control"}),
    )
    handoff_policy = forms.ChoiceField(
        label="Politica de handoff",
        choices=AgentTemplate.HANDOFF_POLICY_CHOICES,
        initial="allow",
        widget=forms.Select(attrs={"class": "form-control form-select"}),
    )
    role = forms.ChoiceField(
        label="Rol",
        choices=AgentTemplate.ROLE_CHOICES,
        initial="phase-worker",
        widget=forms.Select(attrs={"class": "form-control form-select"}),
    )

    def __init__(self, *args, instance: AgentTemplate | None = None, **kwargs):


        super().__init__(*args, instance=instance, **kwargs)
        if instance is not None and not self.is_bound:
            self.fields["slug"].initial = instance.slug
            self.fields["name"].initial = instance.name
            self.fields["system_prompt"].initial = instance.system_prompt
            self.fields["personality"].initial = instance.personality
            self.fields["capabilities"].initial = ", ".join(instance.capabilities or [])
            self.fields["model"].initial = instance.model
            self.fields["temperature"].initial = instance.temperature
            self.fields["max_tokens"].initial = instance.max_tokens
            self.fields["handoff_policy"].initial = instance.handoff_policy
            self.fields["role"].initial = instance.role

    def clean_capabilities(self) -> list[str]:
        raw = self.cleaned_data.get("capabilities", "") or ""
        return [c.strip() for c in raw.split(",") if c.strip()]


class AgentTemplateSearchForm(forms.Form):
    """Busqueda simple en el listado."""

    q = forms.CharField(
        label="Buscar",
        required=False,
        widget=forms.TextInput(attrs={
            "class": "form-control",
            "placeholder": "Nombre, slug o rol...",
        }),
    )
