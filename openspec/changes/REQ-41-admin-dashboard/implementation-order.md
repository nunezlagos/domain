# Implementation Order — REQ-41 admin-dashboard

> **Orden de implementación propuesto** para las 10 HUs hijas de REQ-41, en base a dependencias técnicas internas y externas. Plan revisado en sesión 2026-06-16 (modelo free total → HU-41.6 reescrita como `admin-usage-by-user`).

## Grafo de dependencias internas

```
                    ┌─ 41.1 (shell + sidebar + auth guard)
                    │  ↑
                    │  todas las demás dependen de 41.1
                    │
        ┌── 41.2 (org dashboard) ────────┐
        ├── 41.3 (members + uso)          │
        ├── 41.4 (settings)               │
        ├── 41.6 (usage-by-user) ─────────┤
        ├── 41.5 (audit)                  │
        ├── 41.7 (tickets)                ├── 41.10 (impersonation) ── 41.9 (cross-org)
        ├── 41.8 (cost + tab user)        │
```

## Plan en 5 fases

| Fase | Sprint | HU | Esfuerzo | Paralelizable con | Justificación |
|---|---|---|---|---|---|
| **A. Foundation** | 1-2 | **41.1** | S | — | Sin shell no hay nada. Sidebar admin + routing `/admin/*` + auth guard + runtime config. Bloquea las 9 restantes. |
| **B. Org-facing** | 3-5 | **41.2** | M | 41.3, 41.4 | Vista home. Crea el endpoint nuevo `/admin/org-overview` con `top_users_this_month`. Da el "look & feel" que el admin ve al entrar. |
| **B. Org-facing** | 4-5 | **41.3** | L | 41.2, 41.4 | Members management (la pantalla más usada). Agrega columna "Uso (mes)" con link a HU-41.6. |
| **B. Org-facing** | 4-5 | **41.4** | M | 41.2, 41.3 | Settings simples. Sin tab "Plan" (free total). |
| **C. Operacional** | 5-7 | **41.6** | M | 41.5, 41.7, 41.8 | **NUEVO SCOPE** (era billing, ahora usage-by-user). Endpoint nuevo `/usage/by-user`. Vista central de observabilidad. |
| **C. Operacional** | 5-7 | **41.5** | M | 41.6, 41.7, 41.8 | Audit log viewer. Backend ya está (REQ-34.5). Solo frontend + filtros. |
| **C. Operacional** | 5-7 | **41.7** | M | 41.5, 41.6, 41.8 | Tickets. Tabla ya existe (commit `ff0e0d5`). Formalizar + detalle + bulk. |
| **C. Operacional** | 5-7 | **41.8** | M | 41.5, 41.6, 41.7 | Cost analytics. Agrega tab "Por usuario" (dimension=user). |
| **D. Plataforma** | 7-9 | **41.10** | L | — (prereq de 41.9) | Impersonation. Migración nueva (`impersonation_sessions`) + 3 endpoints + banner sticky. Empezar apenas termine Fase B. |
| **D. Plataforma** | 8-9 | **41.9** | L | — | Cross-org. Depende de 41.10. Crea endpoints `/admin/cross-org-stats` y `/admin/system-health`. |

**Total: 8-9 sprints ≈ 8-9 semanas** con 1 dev full-time. Con 2 devs en paralelo (especialmente en Fase B y C), se puede bajar a **5-6 semanas**.

## Cambio de scope reciente (2026-06-16)

- **HU-41.6**: renombrada de `admin-billing-and-usage` → `admin-usage-by-user`. Decisión: modelo **free total**, sin planes/tiers/billing/Stripe (REQ-21.4 ya está archived).
- **Esfuerzo de 41.6 baja** de L → M (sin integración Stripe, sin invoices, sin upgrade).
- **HU-41.2**: el widget "Plan y uso" se reemplaza por "Top 5 users del mes".
- **HU-41.3**: la tabla de members agrega columna "Uso (mes)".
- **HU-41.4**: ya NO tiene tab "Plan" (modelo free).
- **HU-41.8**: tab "Breakdown" ahora incluye dimensión `user`.
- **HU-41.9**: tabla de orgs NO tiene columna "Plan" (se reemplaza por "Uso total mes").

## Endpoints nuevos a crear (consolidado)

| Endpoint | HU | Notas |
|---|---|---|
| `GET /api/v1/admin/org-overview` | 41.2 | Incluye `top_users_this_month` |
| `GET /api/v1/usage/by-user` | 41.6 | Lista de users de la org con métricas del período |
| `GET /api/v1/usage/by-user/{user_id}` | 41.6 | Drill-down con breakdowns por modelo y proyecto |
| `GET /api/v1/usage/by-user/export` | 41.6 | CSV streaming |
| `GET /api/v1/cost/breakdown/user` | 41.8 | Extensión del `/breakdown/{dimension}` actual — **VERIFICAR** si el handler acepta `dimension=user` |
| `GET /api/v1/admin/cross-org-stats` | 41.9 | Métricas agregadas cross-org (super_admin only) |
| `GET /api/v1/admin/system-health` | 41.9 | Health completo cross-org (super_admin only) |
| `POST /api/v1/admin/impersonate` | 41.10 | Inicia impersonation |
| `POST /api/v1/admin/impersonate/stop` | 41.10 | Sale de impersonation |
| `GET /api/v1/admin/impersonate/active` | 41.10 | Estado de impersonation (para banner) |
| Migración `impersonation_sessions` | 41.10 | Schema nuevo |

## Riesgos del plan

| Riesgo | Mitigación |
|---|---|
| 41.1 se atrasa → todo se atrasa | 41.1 es la única secuencial estricta. Después, paralelizar al máximo. |
| 41.10 tiene migración nueva → más riesgo | Hacer spike técnico de la migración ANTES de estimar (1-2 días). Empezar apenas termine Fase B. |
| 41.9 depende de 41.10 → path crítico | Empezar 41.10 en paralelo con Fase C (no esperar a Fase D). |
| `/usage/by-user` puede ser lento con mucho `token_usage` | Índices en `token_usage(user_id, created_at)`. Caching opcional con TTL 30s. |
| HU-41.6 (usage) tiene más riesgo de lo estimado | Si se atrasa, bloqueará a 41.3 (que linkea al drill-down) y 41.8 (tab user). Empezar 41.6 lo antes posible. |

## Orden recomendado para arrancar

1. **Sprint 1-2**: HU-41.1 (foundation)
2. **Sprint 3**: HU-41.2 (org dashboard) — da la cara del admin
3. **Sprint 4** (paralelo con 2 devs): HU-41.3 + HU-41.4
4. **Sprint 4-5** (paralelo con 2-3 devs): HU-41.6 + HU-41.5 + HU-41.7 + HU-41.8 (las 4 operacionales)
5. **Sprint 6-7**: HU-41.10 (impersonation)
6. **Sprint 7-8**: HU-41.9 (cross-org)

## Criterios de "done" globales para el REQ-41

- [ ] Todas las HUs implementadas y testeadas (unit + integration)
- [ ] E2E del admin funcionando: login OTP → dashboard → members → audit → tickets → cost
- [ ] E2E del super_admin: login → cross-org → impersonation → banner → salir
- [ ] Sidebar de stock (Theme/Colors/Buttons/Forms/Icons/Charts/Widgets/Pages) eliminado del nav
- [ ] Sabotaje tests pasan (romper el cambio → test cae → restaurar)
- [ ] Audit log registra TODA acción administrativa
- [ ] No hay UI de billing/plan/upgrade (modelo free)
- [ ] Docker image `domain-admin:local` se construye sin warnings
- [ ] Caddyfile rutea `/admin/*` al container `domain-admin`
- [ ] Documentación actualizada (README del admin)
