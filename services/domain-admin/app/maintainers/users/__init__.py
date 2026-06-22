"""Mantenedor de usuarios — primer app migrado al patron consolidado `core`.

Es el PATRON DE REFERENCIA: reusa core.models (SoftDeleteModel),
core.service.MaintainerService (list + signal), core.views.MaintainerViews
(las 7 vistas estandar), core.urls.maintainer_urlpatterns y core.forms
(EmailNormalizationMixin). Solo conserva lo propio del dominio: roles,
password hasheado y las rutas extra de asignar/revocar rol.

Django 5.1 autodescubre el AppConfig (apps.UsersConfig) al apuntar
INSTALLED_APPS a "maintainers.users"; no se usa default_app_config
(removido en Django 4.1+).
"""
