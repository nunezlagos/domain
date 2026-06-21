"""Mantenedor de Flows — migrado al patrón consolidado `core`.

Reusa core.models (SoftDeleteModel para Flow), core.service.MaintainerService
(list + signal), core.views.MaintainerViews (las 7 vistas estándar),
core.urls.maintainer_urlpatterns y core.forms (SlugNormalizationMixin). Solo
conserva lo propio del dominio: el spec JSONB, el toggle sobre el boolean
is_active (en vez de status) y las versiones (flow_versions) read-only que se
muestran en el detalle.

Django 5.1 autodescubre el AppConfig (apps.FlowsConfig) al apuntar
INSTALLED_APPS a "maintainers.flows"; no se usa default_app_config
(removido en Django 4.1+).
"""
