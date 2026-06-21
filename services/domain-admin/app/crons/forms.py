"""Forms del mantenedor de Crons (schedules)."""
import json

from django import forms

from .models import Cron


class CronForm(forms.Form):
    """Form para crear/editar crons.

    Usa forms.Form (no ModelForm) porque el modelo es managed=False.
    El slug es único dentro de la organización; en edición se excluye
    el propio registro de la validación de unicidad.

    `inputs` (jsonb) se ingresa como texto JSON y se parsea en clean_inputs.
    `enabled` es un checkbox booleano (toggle on/off del schedule).
    """

    organization_id = forms.UUIDField(
        label="Organización (UUID)",
        widget=forms.TextInput(attrs={"class": "form-control", "autocomplete": "off"}),
        help_text="UUID de la organización dueña de este cron.",
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
    description = forms.CharField(
        label="Descripción",
        required=False,
        widget=forms.Textarea(attrs={"class": "form-control", "rows": 2}),
    )
    cron_expression = forms.CharField(
        label="Expresión cron",
        max_length=100,
        widget=forms.TextInput(attrs={"class": "form-control", "autocomplete": "off",
                                      "placeholder": "0 9 * * *"}),
        help_text="Expresión cron estándar (min hora día mes día-semana).",
    )
    timezone = forms.CharField(
        label="Timezone",
        max_length=50,
        required=False,
        initial="UTC",
        widget=forms.TextInput(attrs={"class": "form-control"}),
        help_text="Zona horaria IANA (ej. UTC, America/Santiago).",
    )
    target_type = forms.ChoiceField(
        label="Tipo de target",
        choices=Cron.TARGET_TYPE_CHOICES,
        initial="flow",
        widget=forms.Select(attrs={"class": "form-control form-select"}),
    )
    target_id = forms.UUIDField(
        label="Target (UUID)",
        widget=forms.TextInput(attrs={"class": "form-control", "autocomplete": "off"}),
        help_text="UUID del flow/agent/skill a disparar.",
    )
    inputs = forms.CharField(
        label="Inputs (JSON)",
        required=False,
        widget=forms.Textarea(attrs={"class": "form-control", "rows": 3,
                                     "placeholder": "{}"}),
        help_text="Objeto JSON con los inputs del target.",
    )
    enabled = forms.BooleanField(
        label="Habilitado",
        required=False,
        initial=True,
        widget=forms.CheckboxInput(attrs={"class": "form-check-input"}),
    )

    def __init__(self, *args, instance: Cron | None = None, **kwargs):
        super().__init__(*args, **kwargs)
        # clean_slug() consulta self.instance para excluirse en edición.
        self.instance = instance
        if instance is not None:
            # organization_id no se edita una vez creado el cron.
            self.fields["organization_id"].required = False
            self.fields["organization_id"].widget.attrs["readonly"] = True
            # Valores iniciales solo al renderizar (unbound).
            if not self.is_bound:
                self.fields["organization_id"].initial = instance.organization_id
                self.fields["name"].initial = instance.name
                self.fields["slug"].initial = instance.slug
                self.fields["description"].initial = instance.description
                self.fields["cron_expression"].initial = instance.cron_expression
                self.fields["timezone"].initial = instance.timezone
                self.fields["target_type"].initial = instance.target_type
                self.fields["target_id"].initial = instance.target_id
                self.fields["inputs"].initial = json.dumps(instance.inputs or {})
                self.fields["enabled"].initial = instance.enabled

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
            qs = Cron.objects.filter(organization_id=org_id, slug=slug)
            if self.instance is not None:
                qs = qs.exclude(pk=self.instance.pk)
            if qs.exists():
                raise forms.ValidationError(
                    "Ya existe un cron con ese slug en esta organización."
                )
        return slug

    def clean_inputs(self):
        raw = (self.cleaned_data.get("inputs") or "").strip()
        if not raw:
            return {}
        try:
            parsed = json.loads(raw)
        except (ValueError, TypeError) as exc:
            raise forms.ValidationError("Inputs debe ser JSON válido.") from exc
        if not isinstance(parsed, dict):
            raise forms.ValidationError("Inputs debe ser un objeto JSON.")
        return parsed

    def clean_timezone(self):
        return (self.cleaned_data.get("timezone") or "").strip() or "UTC"


class CronSearchForm(forms.Form):
    """Búsqueda simple en el listado."""

    q = forms.CharField(
        label="Buscar",
        required=False,
        widget=forms.TextInput(attrs={
            "class": "form-control",
            "placeholder": "Nombre, slug, expresión o target...",
        }),
    )
