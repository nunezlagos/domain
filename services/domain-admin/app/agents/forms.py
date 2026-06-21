"""Forms del mantenedor de Agentes."""
from django import forms

from .models import Agent


class AgentForm(forms.Form):
    """Form para crear/editar agentes.

    Usa forms.Form (no ModelForm) porque el modelo es managed=False.
    El slug es único dentro de la organización; en edición se excluye
    el propio registro de la validación de unicidad.

    `skills_slugs` (text[] en la BD) se edita como texto separado por comas
    y se normaliza a lista en clean_skills_slugs().
    """

    organization_id = forms.UUIDField(
        label="Organización (UUID)",
        widget=forms.TextInput(attrs={"class": "form-control", "autocomplete": "off"}),
        help_text="UUID de la organización dueña de este agente.",
    )
    name = forms.CharField(
        label="Nombre",
        max_length=255,
        widget=forms.TextInput(attrs={"class": "form-control"}),
    )
    slug = forms.SlugField(
        label="Slug",
        max_length=100,
        widget=forms.TextInput(attrs={"class": "form-control", "autocomplete": "off"}),
        help_text="Identificador único dentro de la organización (minúsculas, guiones).",
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
        super().__init__(*args, **kwargs)
        # clean_slug() consulta self.instance para excluirse en edición.
        self.instance = instance
        if instance is not None:
            # organization_id no se edita una vez creado el agente.
            self.fields["organization_id"].required = False
            self.fields["organization_id"].widget.attrs["readonly"] = True
            # Valores iniciales solo al renderizar (unbound).
            if not self.is_bound:
                self.fields["organization_id"].initial = instance.organization_id
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

    def clean_slug(self):
        slug = self.cleaned_data["slug"].strip().lower()
        # La unicidad real es (organization_id, slug). En edición la org no
        # cambia, así que validamos contra la org del instance; en alta usamos
        # el organization_id enviado en el form.
        if self.instance is not None:
            org_id = self.instance.organization_id
        else:
            org_id = self.data.get("organization_id")
        if org_id:
            qs = Agent.objects.filter(organization_id=org_id, slug=slug)
            if self.instance is not None:
                qs = qs.exclude(pk=self.instance.pk)
            if qs.exists():
                raise forms.ValidationError(
                    "Ya existe un agente con ese slug en esta organización."
                )
        return slug

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
