"""Forms del mantenedor de usuarios (migrados a core).

UserForm reusa core.forms.EmailNormalizationMixin para la normalización
strip+lower + unicidad del email (excluyendo la propia instancia en edición).
La lógica propia —choices de roles desde BD, password requerido en alta y
hasheo PBKDF2— queda acá.
"""
from django import forms
from django.contrib.auth.hashers import make_password

from core.forms import EmailNormalizationMixin

from .models import Role, User, UserRole


class UserForm(EmailNormalizationMixin, forms.Form):
    """Form para crear/editar usuarios. Password opcional en edición."""

    # core.forms.EmailNormalizationMixin usa estos atributos para la unicidad.
    email_model = User
    email_field = "email"

    email = forms.EmailField(
        label="Email",
        max_length=255,
        widget=forms.EmailInput(attrs={"class": "form-control", "autocomplete": "off"}),
    )
    name = forms.CharField(
        label="Nombre completo",
        max_length=200,
        required=False,
        widget=forms.TextInput(attrs={"class": "form-control"}),
    )
    role = forms.ChoiceField(
        label="Rol principal",
        choices=[],  # se completa en __init__
        widget=forms.Select(attrs={"class": "form-control form-select"}),
        help_text="Determina los permisos por defecto del usuario.",
    )
    status = forms.ChoiceField(
        label="Estado",
        choices=User.STATUS_CHOICES,
        initial="active",
        widget=forms.Select(attrs={"class": "form-control form-select"}),
    )
    password = forms.CharField(
        label="Contraseña",
        required=False,
        widget=forms.PasswordInput(attrs={"class": "form-control", "autocomplete": "new-password"}),
        help_text="Déjala vacía para mantener la actual (en edición). Mínimo 8 caracteres.",
    )
    password_confirm = forms.CharField(
        label="Confirmar contraseña",
        required=False,
        widget=forms.PasswordInput(attrs={"class": "form-control", "autocomplete": "new-password"}),
    )

    def __init__(self, *args, instance: User | None = None, **kwargs):
        # InstanceAwareMixin (via EmailNormalizationMixin) captura instance.
        super().__init__(*args, instance=instance, **kwargs)
        # Choices de roles desde la DB (roles fijos/seeded).
        self.fields["role"].choices = [
            (r.slug, f"{r.name} ({r.slug})")
            for r in Role.objects.filter(status="active").order_by("slug")
        ]
        # Password requerido en alta, opcional en edición (vacío = no cambiar).
        # Se setea SIEMPRE (también en form bound), si no un POST de alta sin
        # password pasaría la validación.
        is_create = instance is None
        self.fields["password"].required = is_create
        self.fields["password_confirm"].required = is_create
        # Valores iniciales solo al renderizar el form de edición (unbound).
        if instance is not None and not self.is_bound:
            self.fields["email"].initial = instance.email
            self.fields["name"].initial = instance.name
            self.fields["role"].initial = instance.role
            self.fields["status"].initial = instance.status

    def clean(self):
        cleaned = super().clean()
        pw = cleaned.get("password")
        pw_confirm = cleaned.get("password_confirm")

        if pw or pw_confirm:
            if pw != pw_confirm:
                raise forms.ValidationError("Las contraseñas no coinciden.")
            if len(pw) < 8:
                raise forms.ValidationError("La contraseña debe tener al menos 8 caracteres.")
        return cleaned

    def hashed_password(self) -> bytes | None:
        """Devuelve el password hasheado o None si no se cambió."""
        pw = self.cleaned_data.get("password")
        if not pw:
            return None
        return make_password(pw).encode("utf-8")


class UserRoleAssignForm(forms.Form):
    """Form para asignar un rol a un user."""

    role = forms.ModelChoiceField(
        label="Rol",
        queryset=Role.objects.filter(status="active").order_by("slug"),
        widget=forms.Select(attrs={"class": "form-control form-select"}),
    )

    def __init__(self, *args, user: User | None = None, **kwargs):
        super().__init__(*args, **kwargs)
        self.user = user
        if user is not None:
            assigned = UserRole.objects.filter(user=user).values_list("role_id", flat=True)
            self.fields["role"].queryset = (
                Role.objects.filter(status="active").exclude(id__in=assigned).order_by("slug")
            )


class UserSearchForm(forms.Form):
    """Búsqueda simple en el listado."""

    q = forms.CharField(
        label="Buscar",
        required=False,
        widget=forms.TextInput(attrs={
            "class": "form-control",
            "placeholder": "Email o nombre...",
        }),
    )
