"""Mantenedor de API Keys — migrado al patrón consolidado `core`.

Reusa core.models.BaseModel (id/created_at/updated_at), core.service.
MaintainerService (list + signal), core.views.MaintainerViews (las 7 vistas
estándar) y core.urls.maintainer_urlpatterns. Solo conserva lo propio del
dominio: generación del secreto (prefix + hash sha256), soft-delete sobre
`revoked_at` (NO `deleted_at`, que esta tabla no tiene) y el toggle
active<->revoked que limpia/setea `revoked_at`.

NO hereda de SoftDeleteModel porque `auth_api_keys` NO tiene la columna
`deleted_at`: su soft-delete es `revoked_at` + status='revoked'. Heredar
SoftDeleteModel declararía `deleted_at` y dispararía el guard de schema drift.

Django 5.1 autodescubre el AppConfig (apps.ApikeysConfig) al apuntar
INSTALLED_APPS a "maintainers.apikeys".
"""
