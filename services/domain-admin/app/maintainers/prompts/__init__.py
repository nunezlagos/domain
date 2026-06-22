"""Mantenedor de Prompts — migrado al patron consolidado `core`.

Reusa core.models (SoftDeleteModel), core.service.MaintainerService (list +
signal), core.views.MaintainerViews (las 7 vistas estandar),
core.urls.maintainer_urlpatterns y core.forms (SlugNormalizationMixin /
InstanceAwareMixin). Solo conserva lo propio del dominio: unicidad de la
tripleta (project_id, slug, version), tags como lista, y el toggle de
is_active (bool) que reactiva soft-deleted.

Django 5.1 autodescubre el AppConfig (apps.PromptsConfig) al apuntar
INSTALLED_APPS a "maintainers.prompts"; el app_label queda en "prompts"
(ultimo segmento) para mantener {% url 'prompts:...' %} y el guard de
schema drift.
"""
