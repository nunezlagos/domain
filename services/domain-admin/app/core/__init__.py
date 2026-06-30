"""Paquete TRANSVERSAL del admin Django.

`core` extrae el patron comun de los mantenedores (users, projects, apikeys,
agents, skills, flows, crons, prompts) que hoy esta duplicado app por app:

- auth helpers (require_auth / is_ajax)
- modelos base abstractos (BaseModel / SoftDeleteModel) para managed=False
- service base generico (MaintainerService: list + list_signal)
- views/mixins genericos para armar las vistas de un mantenedor
- helper de urlpatterns estandar
- mixins de forms (normalizacion slug/email)

Diseño: NO contiene logica de ningun dominio concreto. Cada app declara su
model/form/service/templates/search_fields y reusa lo de aqui. El contrato de
comportamiento es EXACTAMENTE el del app `users`.
"""
