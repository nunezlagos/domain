"""URL routing del mantenedor de Prompts (migrado a core).

Mounted at /prompts/ en config/urls.py. Las 7 rutas estándar (list, signal,
create, detail, edit, delete, toggle) las arma core.urls.maintainer_urlpatterns
a partir de la instancia `views`. Prompts no tiene rutas propias extra.

app_name="prompts" -> {% url 'prompts:list' %} sigue funcionando igual que antes.
id_kwarg="prompt_id" debe coincidir con el id_kwarg del MaintainerViews.
"""
from core.urls import maintainer_urlpatterns

from . import views

app_name = "prompts"

urlpatterns = maintainer_urlpatterns(views.views, id_kwarg="prompt_id")
