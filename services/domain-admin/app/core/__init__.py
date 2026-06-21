"""Paquete TRANSVERSAL del admin Django.

`core` extrae el patrón común de los mantenedores (users, projects, apikeys,
agents, skills, flows, crons, prompts) que hoy está duplicado app por app:

- auth helpers (require_auth / is_ajax)
- modelos base abstractos (BaseModel / SoftDeleteModel) para managed=False
- service base genérico (MaintainerService: list + list_signal)
- views/mixins genéricos para armar las vistas de un mantenedor
- helper de urlpatterns estándar
- mixins de forms (normalización slug/email)

Diseño: NO contiene lógica de ningún dominio concreto. Cada app declara su
model/form/service/templates/search_fields y reúsa lo de acá. El contrato de
comportamiento es EXACTAMENTE el del app `users`.
"""
