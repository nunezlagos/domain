# REQ-21-org-billing: Organización y billing: gestión de members, invitaciones, planes, límites de uso, integración Stripe.

**Estado:** activo
**Creado:** 2026-06-06
**Fase:** F1, F3

## Descripción

Capa de gestión multi-tenant a nivel organización: invitar y administrar miembros, planes (Free / Pro / Enterprise) con límites de uso (tokens, runs, storage, members), e integración Stripe para billing en planes pagos. Postergable a fase post-MVP pero formalizada ahora para evitar refactors.

## Criterios de éxito

- CRUD de organizaciones con owner, settings (timezone, default model, default channel)
- Invitaciones por email con token de un solo uso, expiración, accept/decline, role asignado
- Planes definidos con límites por dimensión: tokens/mes, runs/mes, storage GB, members, seats
- Tracking de consumo vs límite con throttle/block al exceder (config por org)
- Integración Stripe (Checkout, Webhooks) para upgrade/downgrade, métodos de pago, facturación

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| issue-21.1-org-management | proposed | CRUD orgs, settings, transfer ownership, member roles |
| issue-21.2-user-invitations | proposed | Invitaciones email con token, expiración, accept/decline, audit |
| issue-21.3-plans-limits | proposed | Plans con cuotas por dimensión, tracking, throttle/block al exceder |
| issue-21.4-billing-stripe | proposed | Integración Stripe: Checkout, Webhooks, métodos de pago, invoices |
| issue-21.5-single-org-collapse | proposed | **Deprecación org**: collapse del surface multi-org → single-org (reversible) |
| issue-21.6-org-schema-decommission | proposed | **Deprecación org**: drop destructivo del schema multi-tenant (org_id, RLS, tabla) |

## Nota de dirección (2026-06-17)

Domain pasa a **single-org**: cada deployment self-hosted atiende a UNA organización.
La gestión multi-tenant a nivel aplicación (crear/borrar/transferir orgs, invitaciones
cross-org) se deprecia. Las HUs 21.5 (collapse de surface, reversible) y 21.6
(decommission destructivo del schema, staged) implementan esta deprecación.
Continúa la línea de issue-02.8 (drop custom_roles).
