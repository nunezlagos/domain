"""Mantenedor de Agentes — migrado al patrón consolidado `core`.

Reusa core.models (SoftDeleteModel), core.service.MaintainerService (list +
signal), core.views.MaintainerViews (las vistas estándar),
core.urls.maintainer_urlpatterns y core.forms (SlugNormalizationMixin). Solo
conserva lo propio del dominio: el slug único, skills_slugs como CSV y las
tablas READ-ONLY agent_versions / agent_templates expuestas en el detalle.

A diferencia de users NO hay toggle (no se cablea esa ruta): el listado
excluye los soft-deleted y el detalle anexa versiones + templates.

Django 5.1 autodescubre el AppConfig (apps.AgentsConfig) al apuntar
INSTALLED_APPS a "maintainers.agents".
"""
