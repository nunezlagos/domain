"""Paquete contenedor de los mantenedores del admin.

Agrupa los apps de mantenedor (users, y a futuro projects, apikeys, etc.) bajo
`maintainers.*` para que todos reusen `core` (models/service/views/urls/forms
base) sin duplicar el patron en cada app.

NOTA sobre app_label: el AppConfig de cada sub-app usa `name="maintainers.users"`
pero el `app_label` por defecto queda en el ULTIMO segmento (`users`). Eso es
intencional: mantiene `{% url 'users:...' %}` y el guard de schema drift
(core/tests/test_schema_drift.py espera label "users") funcionando sin cambios.
"""
