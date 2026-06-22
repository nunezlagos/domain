"""URL routing del mantenedor de Skills (migrado a core).

Mounted at /skills/ en config/urls.py. Las 7 rutas estandar (list, signal,
create, detail, edit, delete, toggle) las arma core.urls.maintainer_urlpatterns
a partir de la instancia `views`. skills no expone un boton de toggle en la UI
(la baja es soft-delete via deleted_at), pero la ruta queda cableada por el
helper igual que en el resto de los mantenedores; simplemente no se usa.

app_name="skills" -> {% url 'skills:list' %} sigue funcionando igual que antes.
id_kwarg="skill_id" debe coincidir con el id_kwarg del MaintainerViews.
"""
from core.urls import maintainer_urlpatterns

from . import views

app_name = "skills"

urlpatterns = maintainer_urlpatterns(views.views, id_kwarg="skill_id")
