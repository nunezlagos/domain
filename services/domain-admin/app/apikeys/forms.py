"""HU-API.1: Forms del mantenedor de API Keys.

Se usa forms.Form (no ModelForm) porque el modelo es managed=False.
- En create: se elige el user dueño + nombre + expiración opcional.
- En edit: NO se reasigna el user ni se regenera el secreto; solo se
  editan nombre, expiración y status.
"""
from django import forms

from users.models import User

from .models import ApiKey


class ApiKeyForm(forms.Form):
    """Form para crear/editar API keys."""

    name = forms.CharField(
        label="Nombre",
        max_length=255,
        widget=forms.TextInput(attrs={
            "class": "form-control",
            "placeholder": "Ej: Integración CI/CD",
            "autocomplete": "off",
        }),
        help_text="Etiqueta legible para identificar la key.",
    )
    user = forms.ChoiceField(
        label="Usuario dueño",
        choices=[],  # se completa en __init__
        widget=forms.Select(attrs={"class": "form-control form-select"}),
        help_text="Usuario al que pertenece la API Key.",
    )
    expires_at = forms.DateTimeField(
        label="Expira el",
        required=False,
        widget=forms.DateTimeInput(attrs={
            "class": "form-control",
            "type": "datetime-local",
        }, format="%Y-%m-%dT%H:%M"),
        input_formats=["%Y-%m-%dT%H:%M", "%Y-%m-%d %H:%M:%S", "%Y-%m-%d"],
        help_text="Dejala vacía para que no expire.",
    )
    status = forms.ChoiceField(
        label="Estado",
        choices=ApiKey.STATUS_CHOICES,
        initial="active",
        widget=forms.Select(attrs={"class": "form-control form-select"}),
    )

    def __init__(self, *args, instance: ApiKey | None = None, **kwargs):
        super().__init__(*args, **kwargs)
        self.instance = instance

        # Choices de usuarios activos (dueño de la key).
        self.fields["user"].choices = [
            (str(u.pk), u.display_name)
            for u in User.objects.filter(status="active").order_by("email")
        ]

        # En edición el dueño NO se puede cambiar (regla de negocio): se
        # bloquea el select y se rellena con el dueño actual.
        if instance is not None:
            self.fields["user"].required = False
            self.fields["user"].widget.attrs["disabled"] = "disabled"
            self.fields["user"].choices = [
                (str(instance.user.pk), instance.user.display_name)
            ]
            if not self.is_bound:
                self.fields["name"].initial = instance.name
                self.fields["user"].initial = str(instance.user.pk)
                self.fields["status"].initial = instance.status
                self.fields["expires_at"].initial = instance.expires_at

    def clean_name(self):
        name = self.cleaned_data["name"].strip()
        qs = ApiKey.objects.filter(name=name)
        if self.instance is not None:
            qs = qs.filter(organization_id=self.instance.organization_id).exclude(
                pk=self.instance.pk
            )
        if qs.exists():
            raise forms.ValidationError("Ya existe una API Key con ese nombre.")
        return name

    def clean_user(self):
        # En edición el campo viene disabled (sin valor en POST): conservamos
        # el dueño original.
        if self.instance is not None:
            return str(self.instance.user.pk)
        user_id = self.cleaned_data.get("user")
        if not user_id:
            raise forms.ValidationError("Debés seleccionar un usuario dueño.")
        if not User.objects.filter(pk=user_id).exists():
            raise forms.ValidationError("El usuario seleccionado no existe.")
        return user_id


class ApiKeySearchForm(forms.Form):
    """Búsqueda simple en el listado."""

    q = forms.CharField(
        label="Buscar",
        required=False,
        widget=forms.TextInput(attrs={
            "class": "form-control",
            "placeholder": "Nombre o prefijo...",
        }),
    )
