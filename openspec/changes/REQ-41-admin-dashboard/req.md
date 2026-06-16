# REQ-41-admin-dashboard: Panel web administrativo para gestión operacional de la plataforma (Angular 21 + CoreUI, separado del backend).

**Estado:** activo
**Creado:** 2026-06-16
**Fase:** F4

## Descripción

Panel web SPA servido desde `services/domain-admin` (Angular 21 + CoreUI 5.6, nginx, detrás de Caddy) que consume el API de `domain-backend` y permite a usuarios con rol `admin`, `owner` o `super_admin` gestionar su organización (miembros, settings, audit, billing, tickets, cost) y, en el caso de `super_admin`, ver y operar cross-org (métricas globales, switcher de org, impersonation). El dashboard convive con REQ-16 (que cubre el panel user-facing de un member) y depende de REQ-32 (auth web + CORS + OpenAPI + SDK TS ya implementados). Las vistas de stock del template CoreUI (Theme/Colors/Buttons/Forms/Icons/Charts/Widgets stock) se ELIMINAN del sidebar — solo se reusan sus componentes (`CardComponent`, `TableDirective`, `ButtonDirective`, `ModalComponent`, etc.) para construir las vistas administrativas reales.

## Decisiones arquitectónicas fijadas

- **Modelo comercial**: **Free total, sin planes, sin tiers, sin billing, sin upgrade, sin invoices**. Decisión del usuario (sesión 2026-06-16). REQ-21.4 (Stripe) ya está archived, alineado con esta decisión.
- **Personas**: `viewer`, `developer`, `admin` (de org), `owner` (de org), `super_admin` (cross-org). El dashboard atiende a los 5 roles; capabilities se filtran por RBAC.
- **Métricas per-user**: el foco de observabilidad es **por usuario** (no por org, no por plan). El admin quiere ver, para cada user, cuánto consume: prompts del mes, tokens, runs, storage, cost estimado. Esto se refleja en HU-41.2 (top users), HU-41.3 (columna "uso mes" en members), HU-41.6 (vista dedicada de usage-by-user), HU-41.8 (tab "Por usuario" en cost), HU-41.9 (uso agregado en cross-org).
- **Deployment**: Multi-tenant (multi-cliente). Unidad de scoping = `org_id`. super_admin tiene org-switcher.
- **Stack UI**: Angular 21 + CoreUI 5.6 (decidido en `ff0e0d5`), standalone components, signals para estado, HttpClient directo al backend (sin SDK TS por ahora — se evalúa en REQ-32.4).
- **Auth**: OTP + cookies/JWT vía `/api/v1/auth/*` (issue-32.1). Selector de rol si el user tiene >1 (ya implementado en `views/pages/login/`).
- **Patrón de vista**: mismo patrón que `views/tickets/tickets.component.ts` — `HttpClient` + `signals` + imports de `@coreui/angular` + `apiBase()` + `AuthService`. Cada vista es standalone component.
- **Backend**: Reusar los 238 handlers ya registrados en `internal/api/handler/api.go`. Crear los nuevos endpoints solo cuando ninguna combinación de los existentes cubra la capacidad. Toda creación de endpoint va con su HU correspondiente y respeta RBAC.
- **Sidebar**: el `_nav.ts` actual (con Theme/Colors/Buttons/Forms/Charts/Icons/Widgets stock) se reemplaza por el nav administrativo (Dashboard, Members, Settings, Usage, Audit, Tickets, Cost, [Cross-org si super_admin]).
- **Convivencia con REQ-16**: REQ-16 = dashboard del member (SUS cosas). REQ-41 = panel del admin (gestión). Mismo backend, UI distinta, deploy distinto (`domain-frontend` vs `domain-admin`).

## Criterios de éxito

- Un admin de org puede invitar, asignar roles, revocar miembros y editar settings de su org sin tocar CLI ni SQL
- Un admin puede ver, para cada user de su org, el consumo del mes (prompts, tokens, runs, storage, cost estimado) con sort y drill-down
- Un super_admin puede ver TODAS las orgs, métricas cross-org, system health, y entrar a cualquier org con audit trail de impersonation
- Toda acción administrativa queda registrada en audit log con `actor`, `org_id`, `target`, `action`, `timestamp` (el endpoint `GET /api/v1/audit-logs` ya existe)
- Health del sistema visible para super_admin (API + DB + LLM providers + cost total plataforma)
- Cero endpoint nuevo que no esté justificado por una capacidad concreta del dashboard
- Cero UI de billing/plan/upgrade/invoices (modelo es free total)
- Las vistas de stock de CoreUI (Theme/Colors/Typography/Buttons/Forms stock) ya NO son accesibles desde el sidebar del admin (son demos, no features de producto)
- Login con OTP + selector de rol funciona sin session storage; tokens en localStorage; refresh automático antes de expirar (ya implementado en issue-32.1)

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| issue-41.1-admin-shell-and-routing | propuesta | Sidebar admin (sustituir el de stock), routing interno, auth guard, runtime-config, header con selector de org/rol y badge de user |
| issue-41.2-admin-org-dashboard | propuesta | Vista home del admin: stats de la org, actividad reciente, system health (super_admin), acciones rápidas, plan/uso |
| issue-41.3-admin-members-management | propuesta | CRUD de members, invitar, asignar/cambiar rol, revocar invitaciones, transferir ownership, ver API keys de cada member |
| issue-41.4-admin-org-settings | propuesta | Settings editables: nombre, slug, timezone, default model, default channel, plan (read-only) |
| issue-41.5-admin-audit-log-viewer | propuesta | Tabla con filtros (actor, recurso, action, rango fechas, org si super_admin), export CSV, vista detalle con diff |
| issue-41.6-admin-usage-by-user | propuesta | Vista de usage por user: para cada miembro de la org, métricas del mes (prompts, tokens, runs, storage, cost estimado). Sort, drill-down, export CSV. Sin billing. |
| issue-41.7-admin-tickets-formalize | propuesta | Tickets CRUD (ya existe `views/tickets/`) + comments thread + status history + link a issue/proyecto |
| issue-41.8-admin-cost-analytics | propuesta | Cost summary hoy/mes/avg, breakdown por agente/proyecto, forecast, budgets, export CSV |
| issue-41.9-admin-super-admin-cross-org | propuesta | Solo super_admin: org switcher, vista global con métricas cross-org, system health endpoint, lista de orgs con sortable + filter |
| issue-41.10-admin-impersonation | propuesta | super_admin entra como user X con banner visible, doble audit (impersonator + impersonated), stop impersonation |

## Endpoints backend nuevos (a crear en HUs específicas)

| Endpoint | HU | Justificación |
|----------|----|----|
| `GET /api/v1/admin/org-overview` | 41.2 | Dashboard del admin en 1 sola request. Hoy hacen falta N requests. |
| `GET /api/v1/admin/system-health` | 41.9 | Health completo (API + DB + LLM providers + queue + storage). Hoy `db-stats` es solo DB. |
| `GET /api/v1/admin/cross-org-stats` | 41.9 | Métricas agregadas cross-org para super_admin. |
| `POST /api/v1/admin/impersonate` | 41.10 | super_admin entra como user X. Devuelve session token del impersonated. |
| `POST /api/v1/admin/impersonate/stop` | 41.10 | Salir de impersonation, restaurar sesión del super_admin. |
| `GET /api/v1/admin/impersonate/active` | 41.10 | Devuelve quién está impersonando actualmente (para banner). |

## Dependencias

- REQ-32-dashboard-readiness (auth web + CORS + OpenAPI) — **implementado**
- REQ-02-auth-security (RBAC + OTP + audit log) — **implementado**
- REQ-21-org-billing (orgs, members, invitations, plans) — **parcialmente implementado**
- REQ-15-cost-observability (endpoints de cost) — **implementado**
- REQ-13-http-api (handlers base) — **implementado**
- REQ-04-opsx-sdd (trazabilidad) — usado para HU drafts (issue-41.7 tickets pueden linkear a HU)

## No-objetivos (fuera de alcance)

- Flow editor visual (cubierto por REQ-16 issue-16.3-web-flow-editor)
- Memory explorer (cubierto por REQ-16 issue-16.5-web-admin-memories)
- Marketplace, plugin system, time-travel debugging, A/B testing prompts (F6+)
- Mobile app nativa (la UI es responsive web)
- Multi-idioma i18n (F4+; empezar en español, sumar inglés si hay demanda)
- Temas custom por org (CoreUI soporta dark/light; v1 solo dark)
- Billing, planes, tiers, upgrade, invoices, Stripe (modelo es free total)

## Orden de implementación

Ver `implementation-order.md` en esta misma carpeta. Plan de 5 fases, 8-9 sprints con 1 dev / 5-6 sprints con 2 devs.
