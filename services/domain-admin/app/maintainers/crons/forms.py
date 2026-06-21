"""Forms del mantenedor de Crons (schedules), migrados a core.

CronForm reusa core.forms.InstanceAwareMixin para capturar `instance=` y
poder excluirse a sí mismo en la validación de unicidad del slug (mismo patrón
que el resto de los mantenedores). La lógica propia —parseo de `inputs` (jsonb),
normalización de timezone y el checkbox `enabled`— queda acá.

Usa forms.Form (no ModelForm) porque el modelo es managed=False.
"""
import json

from django import forms

from core.forms import InstanceAwareMixin

from .models import Cron


class CronForm(InstanceAwareMixin, forms.Form):
    """Form para crear/editar crons.

    El slug es único; en edición se excluye el propio registro (vía
    InstanceAwareMixin._exclude_self). `inputs` (jsonb) se ingresa como texto
    JSON y se parsea en clean_inputs. `enabled` es un checkbox booleano.
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
        # InstanceAwareMixin captura instance y lo expone como self.instance.
        super().__init__(*args, instance=instance, **kwargs)
        # Valores iniciales solo al renderizar el form de edición (unbound).
        if instance is not None and not self.is_bound:
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
        # Unicidad por slug; en edición se excluye el propio registro.
        qs = Cron.objects.filter(slug=slug)
        if self._exclude_self(qs).exists():
            raise forms.ValidationError("Ya existe un cron con ese slug.")
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
