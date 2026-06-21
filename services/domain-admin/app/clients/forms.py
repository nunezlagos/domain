"""Forms del mantenedor de Clientes (mandantes)."""
from django import forms

from .models import Client


class ClientForm(forms.Form):
    """Form para crear/editar clientes.

    Usa forms.Form (no ModelForm) porque el modelo es managed=False.
    El slug es único dentro de la organización; en edición se excluye
    el propio registro de la validación de unicidad.
    """

    organization_id = forms.UUIDField(
        label="Organización (UUID)",
        widget=forms.TextInput(attrs={"class": "form-control", "autocomplete": "off"}),
        help_text="UUID de la organización dueña de este cliente.",
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
    tax_id = forms.CharField(
        label="Tax ID / RUT",
        max_length=50,
        required=False,
        widget=forms.TextInput(attrs={"class": "form-control"}),
    )
    contact_email = forms.EmailField(
        label="Email de contacto",
        max_length=255,
        required=False,
        widget=forms.EmailInput(attrs={"class": "form-control", "autocomplete": "off"}),
    )
    contact_phone = forms.CharField(
        label="Teléfono de contacto",
        max_length=50,
        required=False,
        widget=forms.TextInput(attrs={"class": "form-control"}),
    )
    address = forms.CharField(
        label="Dirección",
        required=False,
        widget=forms.Textarea(attrs={"class": "form-control", "rows": 2}),
    )
    status = forms.ChoiceField(
        label="Estado",
        choices=Client.STATUS_CHOICES,
        initial="active",
        widget=forms.Select(attrs={"class": "form-control form-select"}),
    )

    def __init__(self, *args, instance: Client | None = None, **kwargs):
        super().__init__(*args, **kwargs)
        # clean_slug() consulta self.instance para excluirse en edición.
        self.instance = instance
        if instance is not None:
            # organization_id no se edita una vez creado el cliente.
            self.fields["organization_id"].required = False
            self.fields["organization_id"].widget.attrs["readonly"] = True
            # Valores iniciales solo al renderizar (unbound).
            if not self.is_bound:
                self.fields["organization_id"].initial = instance.organization_id
                self.fields["name"].initial = instance.name
                self.fields["slug"].initial = instance.slug
                self.fields["tax_id"].initial = instance.tax_id
                self.fields["contact_email"].initial = instance.contact_email
                self.fields["contact_phone"].initial = instance.contact_phone
                self.fields["address"].initial = instance.address
                self.fields["status"].initial = instance.status

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
            qs = Client.objects.filter(organization_id=org_id, slug=slug)
            if self.instance is not None:
                qs = qs.exclude(pk=self.instance.pk)
            if qs.exists():
                raise forms.ValidationError(
                    "Ya existe un cliente con ese slug en esta organización."
                )
        return slug


class ClientSearchForm(forms.Form):
    """Búsqueda simple en el listado."""

    q = forms.CharField(
        label="Buscar",
        required=False,
        widget=forms.TextInput(attrs={
            "class": "form-control",
            "placeholder": "Nombre, slug, tax ID o email...",
        }),
    )
