"""Mantenedor de Plantillas de Agentes (agent_templates) — patron consolidado `core`.

Reusa core.models (BaseModel: la tabla agent_templates NO tiene deleted_at, asi
que NO hay soft-delete), core.service.MaintainerService (list + signal),
core.views.MaintainerViews (las 7 vistas estandar, de las cuales este mantenedor
NO usa toggle: la baja es HARD delete y status no se alterna desde la UI),
core.urls.maintainer_urlpatterns y core.forms (SlugNormalizationMixin).

IMPORTANTE — colision de tabla con maintainers.agents:
    La tabla `agent_templates` YA esta mapeada por maintainers.agents.models.
    AgentTemplate (modelo READ-ONLY embebido en el detalle de Agentes).
    Declarar aqui un SEGUNDO modelo managed sobre la MISMA db_table dispara
    models.E028 (hard error de system checks: dos modelos managed → misma tabla)
    y, bajo el runner de tests que flipea managed=True, romperia el arranque de
    TODO el suite. Para respetar el contrato "no tocar maintainers.agents ni
    core", este app define su AgentTemplate como PROXY del modelo de agents:
    comparte tabla y columnas (las columnas REALES siguen viviendo en el modelo
    de agents, que matchea agent_templates SIN deleted_at), pero los proxies
    estan EXENTOS de E028 y de la creacion de tabla en syncdb. El proxy agrega
    SOLO el comportamiento propio del mantenedor (Manager con ordering por name).

Django 5.1 autodescubre el AppConfig (apps.AgentTemplatesConfig) al apuntar
INSTALLED_APPS a "maintainers.agenttemplates"; app_label queda fijado a
"agenttemplates" para no romper {% url 'agenttemplates:...' %}.
"""
