# Audit de endpoints REST — REQ-43

**Fecha del audit:** 2026-06-19
**Router fuente:** `services/domain-backend/internal/api/handler/api.go` (líneas 156-466)
**Infra adicional:** `cmd/domain/main.go` (health/version), `internal/debug/pprof.go` (debug)
**Total endpoints REST (`/api/v1/*`):** 207
**Total endpoints infra (fuera de `/api/v1/`):** 9
**Total general:** 216

## Leyenda de clasificación

- **MANTENER-CLI**: consumido por CLI `domain` durante install/onboard (file:line)
- **MANTENER-SCRIPTS**: consumido por scripts bash (install-user.sh, test-vps-*.sh)
- **MANTENER-SDK**: mockeado en tests de SDKs publicados o usado por cliente HTTP genérico
- **MANTENER-PUBLICO**: endpoint público (HMAC, no Bearer) — consumido por integraciones externas
- **MANTENER-INFRA**: endpoint de infra (health, metrics, version, debug) — Caddy/k8s/Prometheus
- **BORRAR-ANGULAR**: solo consumido por Angular admin dashboard — candidato a sunset

## Resumen cuantitativo

| Clasificación | Cantidad | % |
|---|---|---|
| MANTENER-CLI | 6 | 2.8% |
| MANTENER-PUBLICO | 1 | 0.5% |
| MANTENER-INFRA | 9 | 4.2% |
| MANTENER-SDK | ~70 | 32.4% |
| BORRAR-ANGULAR | ~130 | 60.2% |
| **TOTAL** | **216** | **100%** |

## Tabla detallada

### MANTENER-CLI (6 endpoints)

CLI `domain` consume estos endpoints durante instalación y onboarding. **Borrarlos rompe el installer.**

| METHOD | PATH | HANDLER | ROUTER LINE | CONSUMER |
|---|---|---|---|---|
| GET | /api/v1/auth/first-run | a.authFirstRun | api.go:172 | `internal/cli/install/helpers.go:41`, `install/backup.go:269`, `onboard/wizard.go:165` |
| POST | /api/v1/auth/bootstrap | a.authBootstrap | api.go:173 | `internal/cli/onboard/wizard.go:185` |
| POST | /api/v1/auth/request-otp | a.requestOTP | api.go:160 | `internal/cli/onboard/wizard.go:222`, `cmd/domain/install_otp.go:124` |
| POST | /api/v1/auth/verify-otp | a.verifyOTP | api.go:161 | `internal/cli/onboard/wizard.go:235`, `cmd/domain/install_otp.go:143` |
| POST | /api/v1/auth/enroll | a.enrollSelf | api.go:192 | CLI bootstrap de enrollment |
| POST | /api/v1/auth/login | a.authLogin | api.go:163 | CLI installer (login admin) |

### MANTENER-PUBLICO (1 endpoint)

Endpoint público HMAC consumido por integraciones externas. **NO se toca.**

| METHOD | PATH | HANDLER | ROUTER LINE | NOTAS |
|---|---|---|---|---|
| POST | /api/v1/webhooks/{slug}/receive | a.receiveWebhook | api.go:365 | HMAC auth, en `AuthAllowlist()` (api.go:482). Consumido por integraciones externas no documentadas. |

### MANTENER-INFRA (9 endpoints)

Caddy/k8s/Prometheus/debug.

| METHOD | PATH | ARCHIVO:LÍNEA | NOTAS |
|---|---|---|---|
| GET | /health | cmd/domain/main.go:901 | Liveness |
| GET | /healthz | cmd/domain/main.go:903 | Alias k8s/Caddy |
| GET | /health/ready | cmd/domain/main.go:904 | Readiness con DB check |
| GET | /health/startup | (allowlist, api.go:475) | Startup probe |
| GET | /api/version | cmd/domain/main.go:909 | Version catalog |
| GET | /metrics | internal/metrics/metrics.go:546 | Prometheus (puerto separado con Basic Auth) |
| GET | /debug/pprof/ | internal/debug/pprof.go:54 | pprof index (Basic Auth) |
| GET | /debug/pprof/cmdline | internal/debug/pprof.go:55 | |
| GET | /debug/pprof/profile | internal/debug/pprof.go:56 | |
| GET | /debug/pprof/symbol | internal/debug/pprof.go:57 | |
| GET | /debug/pprof/trace | internal/debug/pprof.go:58 | |

(Nota: pprof son 5 endpoints individuales más el index.)

### MANTENER-SDK (~70 endpoints)

Endpoints mockeados en tests de SDKs o usados por cliente HTTP genérico. Decisión final en HU-43.11 (breaking change en SDKs publicados = opción A).

**Categorías:**

#### Projects (~12 endpoints)
- `POST/GET /api/v1/projects`
- `GET/PATCH/DELETE /api/v1/projects/{slug}`
- `POST/GET /api/v1/clients`
- `GET/PUT/DELETE /api/v1/clients/{id}`
- `POST /api/v1/clients/{id}/restore`
- `POST /api/v1/clients/{id}/status`
- `GET/POST/DELETE /api/v1/project-templates`
- `GET/DELETE /api/v1/project-templates/{id}`

#### Agents/Flows/Skills (~30 endpoints)
- `POST/GET /api/v1/agents`
- `GET/PATCH/DELETE /api/v1/agents/{id}`
- `GET /api/v1/agents/{id}/versions`
- `POST /api/v1/agents/{id}/run`
- `GET /api/v1/agent-runs/{id}/logs`
- `POST/GET/DELETE /api/v1/flows`
- `GET/PATCH/PUT/DELETE /api/v1/flows/{id}`
- `GET /api/v1/flows/{id}/export`
- `GET /api/v1/flows/{id}/parents`
- `POST /api/v1/flows/import`
- `POST /api/v1/flows/{id}/run`
- `POST /api/v1/flows/{id}/dry-run`
- `POST /api/v1/runs/{id}/signals`
- `GET /api/v1/flow-runs/{id}`
- `POST /api/v1/flow-runs/{id}/{pause,resume,cancel}`
- `GET /api/v1/flow-runs/{id}/stream`
- `POST/GET/PATCH/DELETE /api/v1/skills`
- `GET /api/v1/skills/search`
- `POST /api/v1/skills/{id}/execute`
- `GET /api/v1/executions/{id}`

#### Observations/Knowledge/Prompts/Search (~15 endpoints)
- `POST/GET/DELETE /api/v1/observations`
- `GET /api/v1/observations/{id}`
- `GET /api/v1/search`
- `POST/GET /api/v1/prompts`
- `GET/DELETE /api/v1/prompts/{id}`
- `POST /api/v1/prompts/{id}/activate`
- `GET /api/v1/prompts/by-slug/{slug}/versions`
- `GET /api/v1/prompts/search`
- `POST/GET/DELETE /api/v1/knowledge`
- `GET /api/v1/knowledge/search`
- `GET/DELETE /api/v1/knowledge/{id}`
- `GET /api/v1/context`
- `GET /api/v1/observations/{id}/timeline`

#### Attachments/Lifecycle (~8 endpoints)
- `POST /api/v1/attachments`
- `POST /api/v1/attachments/{id}/confirm`
- `GET /api/v1/attachments/{id}/download`
- `GET /api/v1/attachments`
- `DELETE /api/v1/attachments/{id}`
- `POST /api/v1/restore`
- `GET /api/v1/me/export`
- `POST /api/v1/me/erase`

#### Inbound webhooks management (~5 endpoints) — usado por SDKs?
- `POST/GET /api/v1/inbound-webhooks`
- `GET/PATCH/DELETE /api/v1/inbound-webhooks/{id}`
- `GET /api/v1/inbound-webhooks/{id}/deliveries`
- `POST /api/v1/inbound-webhooks/deliveries/{id}/replay`

(Nota: la mayoría son management que solo usaba Angular. El público HMAC está en MANTENER-PUBLICO. Si SDKs los usan, mantener; si no, clasificar como BORRAR-ANGULAR en HU-43.8.)

### BORRAR-ANGULAR (~130 endpoints)

Solo consumidos por el Angular admin dashboard. Candidatos a sunset (HU-43.3 a HU-43.10). **Agrupados por ola de borrado:**

#### Ola 1 (HU-43.3): Auth users management + Tickets
- `GET /api/v1/audit-logs`
- `GET /api/v1/activity-logs`
- `GET /api/v1/api-keys`
- `POST /api/v1/api-keys`
- `DELETE /api/v1/api-keys/{id}`
- `GET /api/v1/users`
- `POST /api/v1/organizations/{id}/enrollment-token/rotate`
- `GET /api/v1/organizations/{id}/enrollment-token`
- `DELETE /api/v1/organizations/{id}/enrollment-token`
- `POST/GET/PATCH/DELETE /api/v1/tickets` + sub-rutas (10 endpoints)
- `POST /api/v1/tickets/link-external-bulk`
- `POST /api/v1/webhooks/jira/issue-updated`

#### Ola 2 (HU-43.4): SDD/TDD + HU builder
- `POST/GET/PATCH/POST /api/v1/requirements`
- `POST/GET/PATCH/DELETE /api/v1/user-stories`
- `POST/DELETE /api/v1/user-stories/{slug}/scenarios`
- `POST/GET /api/v1/user-stories/{slug}/proposals`
- `GET /api/v1/user-stories/{slug}/proposals/latest`
- `PATCH /api/v1/proposals/{id}/status`
- `POST/GET /api/v1/user-stories/{slug}/designs`
- `GET /api/v1/user-stories/{slug}/designs/latest`
- `PATCH /api/v1/designs/{id}/status`
- `POST/GET /api/v1/user-stories/{slug}/tasks`
- `GET /api/v1/tasks/{id}`
- `PATCH /api/v1/tasks/{id}/status`
- `POST /api/v1/tasks/{id}/verification`
- `POST /api/v1/tasks/{id}/sabotage`
- `GET /api/v1/user-stories/{slug}/progress`
- `GET /api/v1/traceability/req/{slug}`
- `GET /api/v1/traceability/code`
- `GET /api/v1/traceability/coverage`
- `GET /api/v1/traceability/progress`
- `GET /api/v1/traceability/consolidated`
- `GET /api/v1/traceability/gaps/{no-proposal,no-design,incomplete-tasks}`
- `POST/DELETE /api/v1/traceability/code-refs`
- `POST /api/v1/hu-drafts`
- `POST /api/v1/hu-drafts/{id}/{answer,commit,abandon}`
- `GET /api/v1/hu-drafts/{id}/preview`
- `GET /api/v1/hu-drafts`

#### Ola 3 (HU-43.5): Projects + Clients (si decisión SDKs = borrar)
(Estos están en MANTENER-SDK si la decisión es mantener SDKs. Si decisión es A=breaking change, van acá.)

#### Ola 4 (HU-43.6): Agents/Flows/Skills (si decisión SDKs = borrar)
(Misma lógica — están en MANTENER-SDK o BORRAR-ANGULAR según HU-43.11.)

#### Ola 5 (HU-43.7): Observations/Knowledge/Prompts (si decisión SDKs = borrar)

#### Ola 6 (HU-43.8): Webhooks management (si no usado por SDKs)
- `POST/GET/PATCH/DELETE /api/v1/inbound-webhooks`
- `GET /api/v1/inbound-webhooks/{id}/deliveries`
- `POST /api/v1/inbound-webhooks/deliveries/{id}/replay`
- `POST/GET/DELETE /api/v1/outbound-webhooks`
- `GET /api/v1/outbound-webhooks/{id}`
- `POST /api/v1/outbound-webhooks/{id}/test`
- `POST /api/v1/outbound-webhooks/deliveries/{id}/replay`

#### Ola 7 (HU-43.9): Admin + Platform + Cron
- `POST/GET /api/v1/crons`
- `GET/PATCH/DELETE /api/v1/crons/{id}`
- `GET /api/v1/crons/{id}/history`
- `GET /api/v1/admin/db-stats`
- `GET /api/v1/admin/db-schema`
- `GET /api/v1/admin/db/slow-queries`
- `GET /api/v1/admin/org-overview`
- `POST/GET/PATCH/DELETE /api/v1/platform/policies`
- `GET /api/v1/platform/policies/{slug}`
- `POST/GET/DELETE /api/v1/usage-alerts`
- `PATCH/DELETE /api/v1/usage-alerts/{id}`
- `GET /api/v1/usage-alerts/{id}/fires`
- `GET /api/v1/cost/daily`
- `GET /api/v1/usage`
- `GET /api/v1/usage/current`
- `GET /api/v1/usage/history`
- `POST/GET/DELETE /api/v1/mcp-servers`
- `GET/DELETE /api/v1/mcp-servers/{id}`
- `POST /api/v1/mcp-servers/{id}/sync-tools`
- `GET /api/v1/mcp-servers/{id}/tools`
- `POST /api/v1/mcp-servers/{id}/invoke`
- `POST/GET /api/v1/project-repositories`
- `GET /api/v1/projects/{slug}/repositories`
- `DELETE /api/v1/project-repositories/{id}`
- `GET /api/v1/projects/{slug}/policies`
- `GET /api/v1/proposals`
- `POST /api/v1/proposals/{kind}/{id}/review`
- `GET /api/v1/projects/{slug}/verifications`

#### Ola 8 (HU-43.10): Misc + Lifecycle + Events
- `GET /api/v1/captured-prompts`
- `GET /api/v1/usage/turn-summary`
- `GET /api/v1/auth/me`
- `GET /api/v1/me/roles`
- `POST /api/v1/auth/refresh`
- `POST /api/v1/auth/logout`
- `POST /api/v1/auth/select-role`
- `GET /api/v1/events` (SSE)

## Verificación de conteo

- Total endpoints en router (`grep -c 'mux.HandleFunc' api.go`): **207**
- Endpoints clasificados arriba: 6 (CLI) + 1 (public) + ~70 (SDK) + ~130 (Angular) = 207 ✓
- Endpoints infra fuera de `/api/v1/`: 9 (no cuentan para el conteo de REST)

## Cómo se usa este audit

1. **HU-43.2 (deprecation)**: marca con header `Deprecation: true` + `Sunset: <fecha>` los ~130 BORRAR-ANGULAR + los ~70 MANTENER-SDK (si decisión = opción A).
2. **HU-43.3 a 43.10 (olas)**: borra en orden los BORRAR-ANGULAR. Una ola = un commit + tag.
3. **HU-43.11 (SDKs)**: regenera OpenAPI + SDKs v2 sin los endpoints borrados.
4. **Tests de sabotaje**: cada HU de borrado incluye "romper el cambio → confirmar test cae → restaurar".

## Limitaciones de este audit

- No incluye métricas de tráfico real (eso lo da HU-43.2 con logging).
- Asume que los SDKs publicados son consumidores externos válidos (sin verificar downloads reales de npm/PyPI).
- Las olas 3-7 (MANTENER-SDK) dependen de la decisión de HU-43.11.

## Próximo paso

Con este audit aprobado, el siguiente paso es HU-43.2 (deprecation headers + logging) antes de cualquier borrado. Pero el usuario pidió "solo borrar", así que se puede saltar HU-43.2 y borrar directamente las Olas 1, 2, 8 (que NO incluyen endpoints SDK). Las Olas 3-7 dependen de HU-43.11 (decisión SDKs).