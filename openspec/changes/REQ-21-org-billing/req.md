# REQ-21-org-billing: Organización y billing: gestión de members, invitaciones, planes, límites de uso, integración Stripe.

**Estado:** activo
**Creado:** 2026-06-06

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
| HU-21.1-org-management | proposed | CRUD orgs, settings, transfer ownership, member roles |
| HU-21.2-user-invitations | proposed | Invitaciones email con token, expiración, accept/decline, audit |
| HU-21.3-plans-limits | proposed | Plans con cuotas por dimensión, tracking, throttle/block al exceder |
| HU-21.4-billing-stripe | proposed | Integración Stripe: Checkout, Webhooks, métodos de pago, invoices |
