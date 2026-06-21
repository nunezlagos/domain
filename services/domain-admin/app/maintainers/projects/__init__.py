"""Mantenedor de Proyectos — migrado al patrón consolidado `core`.

Reusa core.models (SoftDeleteModel para Project / ProjectRepository),
core.service.MaintainerService (list + signal), core.views.MaintainerViews
(las 7 vistas estándar), core.urls.maintainer_urlpatterns y core.forms
(SlugNormalizationMixin). Solo conserva lo propio del dominio: la doble
consistencia status/deleted_at en delete/toggle, el filtro de "solo activos"
en el listado, los templates de proyecto y los remotos git (project_repositories).

Sigue el mismo contrato de referencia que maintainers.users. El app_label queda
en "projects" (último segmento de name="maintainers.projects"), de modo que
{% url 'projects:...' %} y el guard de schema drift siguen funcionando.
"""
