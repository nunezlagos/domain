"""URL routing del mantenedor de usuarios (migrado a core).

Mounted at /usuarios/ en config/urls.py. Las 7 rutas estándar (list, signal,
create, detail, edit, delete, toggle) las arma core.urls.maintainer_urlpatterns
a partir de la instancia `views`. Se suman las 2 rutas propias del dominio
roles (asignar / revocar).

app_name="users" -> {% url 'users:list' %} sigue funcionando igual que antes.
id_kwarg="user_id" debe coincidir con el id_kwarg del MaintainerViews.
"""
from django.urls import path

from core.urls import maintainer_urlpatterns

from . import views

app_name = "users"

urlpatterns = maintainer_urlpatterns(views.views, id_kwarg="user_id") + [
    path("<uuid:user_id>/roles/asignar/", views.role_assign, name="role_assign"),
    path("<uuid:user_id>/roles/<uuid:role_id>/revocar/", views.role_revoke, name="role_revoke"),
]
