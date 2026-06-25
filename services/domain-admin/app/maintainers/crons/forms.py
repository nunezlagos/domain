"""Forms del mantenedor de Crons (schedules), migrados a core.

CronForm reusa core.forms.InstanceAwareMixin para capturar `instance=` y
poder excluirse a si mismo en la validacion de unicidad del slug (mismo patron
que el resto de los mantenedores). La logica propia —parseo de `inputs` (jsonb),
normalizacion de timezone y el checkbox `enabled`— queda aqui.

Usa forms.Form (no ModelForm) porque el modelo es managed=False.
"""
import json

from django import forms

from core.forms import InstanceAwareMixin

from .models import Cron


class CronForm(InstanceAwareMixin, forms.Form):
    """Form para crear/editar crons.

    El slug es unico; en edicion se excluye el propio registro (via
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
        help_text="Identificador unico (minusculas, guiones).",
    )
    description = forms.CharField(
        label="Descripcion",
        required=False,
        widget=forms.Textarea(attrs={"class": "form-control", "rows": 2}),
    )
    cron_expression = forms.CharField(
        label="Expresion cron",
        max_length=100,
        widget=forms.TextInput(attrs={"class": "form-control", "autocomplete": "off",
                                      "placeholder": "0 9 * * *"}),
        help_text="Expresion cron estandar (min hora dia mes dia-semana).",
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
        widget=forms.Textarea(attrs={"class": "form-control form-control--code", "rows": 10,
                                     "spellcheck": "false", "placeholder": "{}"}),
        help_text="Objeto JSON con los inputs del target (editor monospace).",
    )
    enabled = forms.BooleanField(
        label="Habilitado",
        required=False,
        initial=True,
        widget=forms.CheckboxInput(attrs={"class": "form-check-input"}),
    )

    def __init__(self, *args, instance: Cron | None = None, **kwargs):

        super().__init__(*args, instance=instance, **kwargs)

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
            raise forms.ValidationError("Inputs debe ser JSON valido.") from exc
        if not isinstance(parsed, dict):
            raise forms.ValidationError("Inputs debe ser un objeto JSON.")
        return parsed

    def clean_timezone(self):
        return (self.cleaned_data.get("timezone") or "").strip() or "UTC"


class CronSearchForm(forms.Form):
    """Busqueda simple en el listado."""

    q = forms.CharField(
        label="Buscar",
        required=False,
        widget=forms.TextInput(attrs={
            "class": "form-control",
            "placeholder": "Nombre, slug, expresion o target...",
        }),
    )
