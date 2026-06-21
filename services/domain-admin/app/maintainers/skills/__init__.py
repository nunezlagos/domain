"""Mantenedor de Skills — migrado al patrón consolidado `core`.

Reusa core.models (SoftDeleteModel para skills), core.service.MaintainerService
(list + signal), core.views.MaintainerViews (las 7 vistas estándar, de las
cuales skills NO usa toggle: la baja es soft-delete vía deleted_at),
core.urls.maintainer_urlpatterns y core.forms (SlugNormalizationMixin). Solo
conserva lo propio del dominio: scope de slug por (project_id, slug), parseo de
tags y la lista READ-ONLY de versiones (skill_versions).

Django 5.1 autodescubre el AppConfig (apps.SkillsConfig) al apuntar
INSTALLED_APPS a "maintainers.skills"; app_label queda fijado a "skills" para
no romper {% url 'skills:...' %} ni el guard de schema drift.
"""
