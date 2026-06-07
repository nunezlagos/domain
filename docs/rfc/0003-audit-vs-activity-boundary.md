# RFC 0003: Audit Log vs Activity Log Boundary

**Status:** accepted
**Date:** 2026-06-07
**Related:** HU-02.4 audit-log, HU-02.6 activity-log

## Contexto

REQ-02 incluye dos HUs que suenan similares y se solapan en la práctica:

- **HU-02.4 audit-log**: "Immutable audit trail, queries, retention 90d"
- **HU-02.6 activity-log**: "Activity log general: quién, qué, cuándo, filtrable por proyecto/usuario/entidad"

Sin una boundary clara, el equipo va a duplicar tablas, escribir ambos sistemas para el mismo evento, o (peor) usar uno solo malamente.

## Decisión

Mantenemos AMBAS tablas con propósitos distintos, no fusionar:

### audit_log (HU-02.4) — **technical / compliance**

- **Append-only**, INSERT only, NO UPDATE NO DELETE permitidos (revoke INSERT desde app role).
- **Granularidad alta**: cada cambio de field con `old_values`/`new_values` JSONB.
- **Inmutable**: enforced por DB grants + trigger que rechaza UPDATE.
- **Retention 90 días** default (config), después archive a S3 cold storage.
- **Audience**: auditores, compliance, security incident response.
- **NO user-facing**: no UI directa para usuarios; admin tools only.
- **Datos sensibles permitidos**: `old_values`/`new_values` pueden contener PII (cifrado at-rest, acceso loggeado).
- **Latencia inserción**: best-effort, async OK; pérdida es aceptable solo en catastrophic failure.
- **Tabla**: `audit_log` con `BIGSERIAL id` (alto volumen), `actor_id`, `action`, `entity_type`, `entity_id`, `old_values JSONB`, `new_values JSONB`, `ip_address`, `occurred_at`.

### activity_log (HU-02.6) — **product / UX**

- **Append-only PERO mutable** (puede agregarse correction event)
- **Granularidad humana**: "Alice creó observation X", "Bob ejecutó flow Y", "Carol invitó a Dave"
- **User-facing**: aparece en feeds, timelines, notifications de la UI/API
- **Retention indefinida** (parte del producto)
- **Audience**: usuarios finales, members de la org
- **Sin datos sensibles**: solo metadata sumarizada legible (`"Bob updated project settings"`, NO los diffs)
- **Latencia inserción**: best-effort
- **Tabla**: `activity_log` con UUID id, `organization_id`, `actor_id`, `action`, `entity_type`, `entity_id`, `summary TEXT` (human-readable), `metadata JSONB` (small, NO PII), `created_at`.

## Tabla comparativa

| dimensión | audit_log | activity_log |
|-----------|-----------|--------------|
| Propósito | compliance, forensics | UX, awareness |
| Audience | auditores, security | users, members |
| Mutability | strict append-only (DB-enforced) | append-only (convención) |
| Granularidad | field-level diffs | human summary |
| Datos sensibles | sí (cifrados) | no |
| Retention | 90d → cold storage | indefinida |
| Volumen esperado | alto (todo cambio) | medio (sólo eventos meaningful) |
| Performance escrita | crítico (async OK) | aceptable best-effort |
| UI exposed | no (admin only) | sí (parte del producto) |

## Eventos: ¿cuál uso cuándo?

| evento | audit_log | activity_log |
|--------|-----------|--------------|
| User update settings | ✓ (diff) | ✓ ("Alice updated settings") |
| Login | ✓ (ip, ua, success) | optional (puede saturar) |
| API key rotated | ✓ | ✓ |
| Observation created | optional (sólo si compliance lo pide) | ✓ |
| Agent run completed | optional | ✓ |
| Skill executed internally | no | no (demasiado granular) |
| Cron tick | no | no |
| RBAC permission denied | ✓ | optional |
| Stripe webhook recibido | ✓ | optional |

Regla: **audit_log es opt-out (default ON para changes), activity_log es opt-in (sólo si el user debería verlo)**.

## Implementación

### audit_log

- Generar via trigger PostgreSQL `AFTER UPDATE OR DELETE OR INSERT` sobre tablas relevantes:
  - users, organizations, api_keys, roles, secrets, plans, subscriptions
- Para entidades de dominio (observations, agents, flows): vía middleware en service layer
- Grants: `REVOKE UPDATE, DELETE ON audit_log FROM app_user`

### activity_log

- Generar explícitamente desde service layer con `activity.Record(ctx, summary, ...)`
- Buffered + batch insert cada 5s para reducir overhead
- Renderizado: `summary` ya es human-readable; UI puede aplicar i18n template adicional si necesario

## Anti-patrones

- ❌ Usar activity_log para auditoría de compliance (no es inmutable)
- ❌ Usar audit_log para feed de UI (no escalable, contiene PII)
- ❌ Insertar diff completo de payload grande en activity (debe ser human summary)
- ❌ Duplicar el mismo evento en ambas tablas si no aplica el criterio

## Consecuencias

**Positivas:**
- Compliance (auditores) ven la verdad técnica
- Users (UI) ven una versión legible
- Performance: tabla apropiada para cada workload
- Privacidad: separación clara de qué tiene PII y qué no

**Negativas:**
- Dos tablas en lugar de una (más maintenance)
- Riesgo de "olvidarse" de escribir a una → linter test que valida ciertos events se escriben a ambas

## Open questions

- ¿Cold storage destination? Probablemente S3 con `s3 archive` lifecycle (HU-18.2 cubre).
- ¿Activity log debería tener notificación push (websocket)? Sí, futuro: `activity_feed_subscriptions` table.
