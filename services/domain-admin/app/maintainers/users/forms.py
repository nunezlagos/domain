"""Forms del mantenedor de usuarios (migrados a core).

UserForm reusa core.forms.EmailNormalizationMixin para la normalizacion
strip+lower + unicidad del email (excluyendo la propia instancia en edicion).
La logica propia —choices de roles desde BD, password requerido en alta y
hasheo PBKDF2— queda aqui.
"""
from django import forms
from django.contrib.auth.hashers import make_password

from core.forms import EmailNormalizationMixin

from .models import Role, User, UserRole


class UserForm(EmailNormalizationMixin, forms.Form):
    """Form para crear/editar usuarios. Password opcional en edicion."""


    email_model = User
    email_field = "email"

    email = forms.EmailField(
        label="Email",
        max_length=255,
        widget=forms.EmailInput(attrs={"class": "form-control", "autocomplete": "off"}),
    )
    first_name = forms.CharField(
        label="Nombres",
        max_length=120,
        required=False,
        widget=forms.TextInput(attrs={"class": "form-control"}),
    )
    paternal_surname = forms.CharField(
        label="Apellido paterno",
        max_length=60,
        required=False,
        widget=forms.TextInput(attrs={"class": "form-control"}),
    )
    maternal_surname = forms.CharField(
        label="Apellido materno",
        max_length=60,
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
        help_text="Dejala vacia para mantener la actual (en edicion). Minimo 8 caracteres.",
    )
    password_confirm = forms.CharField(
        label="Confirmar contraseña",
        required=False,
        widget=forms.PasswordInput(attrs={"class": "form-control", "autocomplete": "new-password"}),
    )

    def __init__(self, *args, instance: User | None = None, **kwargs):

        super().__init__(*args, instance=instance, **kwargs)

        self.fields["role"].choices = [
            (r.slug, f"{r.name} ({r.slug})")
            for r in Role.objects.filter(status="active").order_by("slug")
        ]



        is_create = instance is None
        self.fields["password"].required = is_create
        self.fields["password_confirm"].required = is_create

        if instance is not None and not self.is_bound:
            self.fields["email"].initial = instance.email
            first, paternal, maternal = self._split_name(instance.name)
            self.fields["first_name"].initial = first
            self.fields["paternal_surname"].initial = paternal
            self.fields["maternal_surname"].initial = maternal
            self.fields["role"].initial = instance.role
            self.fields["status"].initial = instance.status

    @staticmethod
    def _split_name(full: str) -> tuple[str, str, str]:
        """Parte `name` en (nombres, apellido paterno, apellido materno).

        Heuristica (lossy, el usuario reedita): 1 token -> todo a nombres;
        2 -> nombres + paterno; 3+ -> primero=nombres, ultimo=materno, el
        medio=paterno.
        """
        parts = (full or "").split()
        if not parts:
            return "", "", ""
        if len(parts) == 1:
            return parts[0], "", ""
        if len(parts) == 2:
            return parts[0], parts[1], ""
        return parts[0], " ".join(parts[1:-1]), parts[-1]

    def composed_name(self) -> str:
        """Recompone `name` desde los 3 campos (para mandar al service)."""
        parts = [
            self.cleaned_data.get(k, "").strip()
            for k in ("first_name", "paternal_surname", "maternal_surname")
        ]
        return " ".join(p for p in parts if p)

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
        """Devuelve el password hasheado o None si no se cambio."""
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
    """Busqueda simple en el listado."""

    q = forms.CharField(
        label="Buscar",
        required=False,
        widget=forms.TextInput(attrs={
            "class": "form-control",
            "placeholder": "Email o nombre...",
        }),
    )
