"""Mantenedor de Crons (schedules) — migrado al patrón consolidado `core`.

Reusa core.models (SoftDeleteModel), core.service.MaintainerService (list +
signal), core.views.MaintainerViews (las 7 vistas estándar),
core.urls.maintainer_urlpatterns y core.forms (mixins). Solo conserva lo propio
del dominio: el flag booleano `enabled` (dimensión del toggle, distinta del
`status`), el parseo de `inputs` (jsonb) y la unicidad de `slug`.

Django 5.1 autodescubre el AppConfig (apps.CronsConfig) al apuntar
INSTALLED_APPS a "maintainers.crons". El app_label queda fijado en "crons"
(apps.py) para no romper {% url 'crons:...' %} ni el guard de schema drift.
"""
