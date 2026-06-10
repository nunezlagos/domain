# Domain — Roadmap MVP → Versión Final

**Última actualización:** 2026-06-07
**Total REQs activos:** 27
**Total HUs:** 148

Este roadmap divide la implementación en **6 fases** con criterio de exit explícito. Cada fase entrega valor incremental y deja el sistema en estado deployable.

> **Lectura:** Las fases son secuenciales; las HUs dentro de una fase pueden ejecutarse en paralelo respetando sus dependencias internas.

---

## Visión general por fase

| Fase | Nombre | HUs | Esfuerzo (sem) | Estado deployable | Personas-mes |
|------|--------|-----|----------------|-------------------|--------------|
| **0** | Bootstrap dev environment | 3 | 1 | dev local | 0.5 |
| **1** | Foundation técnica | 22 | 6-8 | smoke testable | 4-6 |
| **2** | MVP funcional (alpha) | 28 | 8-10 | usable internamente | 8-12 |
| **3** | Beta privada | 24 | 6-8 | clientes invitados | 8-10 |
| **4** | Production-ready (v1.0) | 32 | 10-12 | producción B2B | 12-16 |
| **5** | Escala y robustez (v1.x) | 22 | 8-10 | scale-out | 10-14 |
| **6** | Diferenciación (v2.0) | 17 | 10-12 | competitivo enterprise | 12-16 |

**Total estimado**: 9-12 meses con equipo 4-6 devs senior, o 14-18 meses con equipo 2-3 devs.

---

## Fase 0 — Bootstrap dev environment (1 semana)

**Objetivo:** poder correr código localmente con stack completo, sin tocar features de producto.

**HUs:**
- **issue-01.6** local-dev-environment (docker-compose con pg+minio+adminer+mailpit)
- **issue-01.2** config-system (env vars DOMAIN_*, validation al boot)
- **issue-01.3** health-version (CLI + endpoint /health)

**Exit criteria:**
- [ ] `make dev-up` levanta stack completo en <60s
- [ ] `domain version` reporta correctamente
- [ ] `curl http://localhost:8080/health` → 200
- [ ] `.env.example` funcional como template

**Riesgos:** ninguno (infra dev, contained).

---

## Fase 1 — Foundation técnica (6-8 semanas)

**Objetivo:** schema completo, migraciones, seeds, auth, observability básica. Sin features de producto user-facing aún, pero todo el "esqueleto" está armado.

### 1.1 Schema + Migraciones (sem 1-2)

- **issue-01.1** db-schema-migrations (23 tablas iniciales + extensiones)
- **issue-25.3** migration-linter (squawk + custom)
- **issue-25.13** schema-conventions-linter (enforce db.md)
- **issue-25.6** least-privilege-roles (4 roles + grants)
- **issue-01.7** seeders-system (framework + go:embed)

### 1.2 Observabilidad base (sem 3)

- **issue-17.1** metrics-prometheus
- **issue-17.2** tracing-otel
- **issue-17.3** structured-logging-slog

### 1.3 Auth core (sem 4-5)

- **issue-02.1** api-key-auth (bcrypt, CRUD, rotación)
- **issue-02.2** rbac (roles built-in)
- **issue-02.3** secrets-encryption (AES-256-GCM)
- **issue-02.4** audit-log (append-only inmutable)
- **issue-02.5** rate-limit-pii
- **issue-02.6** activity-log
- **issue-02.7** passwordless-otp-auth (RUT/email → API key)

### 1.4 Hardening DB esencial (sem 6)

- **issue-25.5** rls-sensitive-tables (12 tablas críticas)
- **issue-25.8** resource-limits-timeouts (statement_timeout, TLS verify-full, pg_hba)
- **issue-25.1** pgbouncer-pooling (transaction-pool + HA)
- **issue-25.2** pg-stat-statements

### 1.5 Multi-tenant base (sem 7-8)

- **issue-21.1** org-management (CRUD + transfer)
- **issue-21.2** user-invitations (email + accept via OTP)
- **issue-01.4** project-templates
- **issue-01.5** project-merge
- **issue-01.8** platform-policies (rules en BD)

**Exit criteria fase 1:**
- [ ] Smoke E2E: invitar user → recibir email → OTP login → recibir API key → llamar `/health` autenticado
- [ ] CI verde: lint + unit + integration + migration linter + schema conventions
- [ ] 2 pods Postgres+app levantan, métricas + traces visibles, logs estructurados JSON
- [ ] RLS test adversarial: bug RBAC simulado → cross-org leak bloqueado
- [ ] Performance baseline: smoke 100 req/s p99 <100ms

**Riesgos:**
- RLS performance: hacer bench temprano
- PgBouncer + prepared stmts pgx: tener config correcta ANTES de feature work

---

## Fase 2 — MVP funcional alpha (8-10 semanas)

**Objetivo:** features core del producto funcionando. Domain ya es una memoria + ejecutor de agentes con MCP server. Usable internamente por el equipo.

### 2.1 Memoria + búsqueda (sem 9-11)

- **issue-03.1** observations-crud-fts
- **issue-03.2** sessions-lifecycle
- **issue-03.3** prompts-storage
- **issue-03.4** knowledge-documents (chunking + embeddings)
- **issue-03.5** context-timeline
- **issue-03.6** dedup-privacy
- **issue-03.7** cross-project-global-search (FTS+vector híbrida)

### 2.2 LLM + embeddings (sem 12)

- **issue-06.1** llm-provider-factory
- **issue-06.2** llm-runners (OpenAI, Anthropic, Google)
- **issue-06.4** model-registry-cost
- **issue-06.5** embedding-pgvector
- **issue-06.6** token-count-stream

### 2.3 Skills + ejecución (sem 13-14)

- **issue-05.1** skill-definitions
- **issue-05.2** skill-registry-search
- **issue-05.3** skill-versioning
- **issue-05.4** auto-skill-engine
- **issue-05.5** skill-execution
- **issue-05.6** agent-skill-contract (JSON Schema + error taxonomy)

### 2.4 Agentes core (sem 15-16)

- **issue-08.1** agent-definitions
- **issue-08.2** agent-execution (loop LLM + tools)
- **issue-08.3** agent-runs-logs
- **issue-08.5** agent-templates (5 built-in)

### 2.5 MCP server (sem 17)

- **issue-12.1** mcp-core-stdio
- **issue-12.2** mcp-memory-tools (12 tools `domain_mem_*`)
- **issue-12.3** mcp-agent-tools (9 tools `domain_*`)
- **issue-12.5** agent-setup (auto-config Claude/Cline/Cursor)

### 2.6 HTTP API + CLI base (sem 18)

- **issue-13.1** http-crud-endpoints
- **issue-13.2** http-auth-middleware
- **issue-13.3** http-pagination-filters
- **issue-14.1** cli-core-commands
- **issue-14.2** cli-output-formats
- **issue-14.3** cli-autocomplete-help

**Exit criteria fase 2:**
- [ ] Workflow E2E completo: dev usa Claude Code → MCP `domain_mem_save` guarda obs → `domain_agent_run` ejecuta agente con skills → resultado persistido y consultable vía API
- [ ] CLI `domain` permite todas las operaciones core
- [ ] Search global devuelve resultados rankeados con FTS+vector en <500ms
- [ ] 5 templates de agentes built-in funcionan out-of-the-box
- [ ] El equipo interno usa Domain para su propio trabajo (dogfooding)

**Hito de marketing:** alpha cerrado a equipo + ~5 testers de confianza.

**Riesgos:**
- LLM cost runaway en dogfooding: issue-21.3 plans/limits NO está aún en esta fase → poner cap manual fuerte
- Performance search con 10k+ observations: bench y ajustar índices

---

## Fase 3 — Beta privada (6-8 semanas)

**Objetivo:** estabilizar para clientes invitados. Add billing, notifications, costos, observabilidad de producto.

### 3.1 Flow system (sem 19-21)

- **issue-09.1** flow-dag-definition
- **issue-09.2** step-types (10 step types)
- **issue-09.3** flow-state-machine
- **issue-09.4** retry-error-handling
- **issue-09.5** subflows-composition

### 3.2 Cron + Triggers (sem 22)

- **issue-10.1** cron-schedules
- **issue-10.2** webhook-triggers (inbound HMAC)
- **issue-10.3** event-execution

### 3.3 Cost observability (sem 23)

- **issue-15.1** token-tracking
- **issue-15.2** cost-analytics
- **issue-15.3** usage-alerts

### 3.4 Plans + billing (sem 24-25)

- **issue-21.3** plans-limits (Free/Pro/Enterprise + cuotas)
- **issue-21.4** billing-stripe (Checkout + webhooks)

### 3.5 Notificaciones (sem 26)

- **issue-20.1** channel-abstraction
- **issue-20.2** email-smtp
- **issue-20.3** slack-webhook

### 3.6 MCP avanzado + resilience (sem 27)

- **issue-12.4** mcp-bidirectional (consume MCPs externos)
- **issue-12.6** mcp-tool-resilience (timeout + CB + cache + degraded)

### 3.7 Context/cache LLM (sem 28)

- **issue-07.1** context-optimizer
- **issue-07.2** cross-session-stitch
- **issue-07.3** llm-semantic-cache
- **issue-07.4** token-budget

**Exit criteria fase 3:**
- [ ] Cliente beta puede signup vía invitación, registrar API key, ejecutar agente, ver costo en su dashboard
- [ ] Stripe Checkout funciona end-to-end (upgrade Free → Pro)
- [ ] Alertas funcionan: 80% uso tokens → email + Slack
- [ ] Resilience: caída OpenAI no rompe MCP tools (degraded responses con cache local)
- [ ] Cost analytics granular por agent/flow/user/period

**Hito marketing:** beta privada con 10-30 clientes invitados, NDA opcional.

**Riesgos:**
- Stripe integration: testing exhaustivo en stripe-mock antes de prod keys
- Notification fatigue: throttle defaults conservadores

---

## Fase 4 — Production-ready v1.0 (10-12 semanas)

**Objetivo:** GA público. Robusto, escalable, deployable en cualquier cloud, con SDKs.

### 4.1 Backup & DR (sem 29-30)

- **issue-18.1** postgres-backups (pgBackRest + PITR)
- **issue-18.2** s3-replication
- **issue-18.3** restore-runbook + drill mensual

### 4.2 CI/CD producción (sem 31)

- **issue-19.1** ci-lint-test (ya parcial desde fase 1; completar matrix + integration)
- **issue-19.2** cd-release-binary (goreleaser + cosign + SBOM)
- **issue-19.3** docker-image-publish (distroless multi-arch)

### 4.3 Deployment K8s (sem 32-33)

- **issue-24.1** helm-chart (oficial OCI)
- **issue-24.2** kustomize-overlays
- **issue-24.3** k8s-deployment-examples (AWS EKS, GCP GKE, k3s)

### 4.4 SDKs (sem 34-35)

- **issue-22.1** sdk-python
- **issue-22.2** sdk-typescript
- **issue-22.3** sdk-go

### 4.5 API maduro (sem 36)

- **issue-13.4** idempotency-keys
- **issue-13.5** bulk-batch-endpoints
- **issue-13.6** cursor-pagination
- **issue-13.7** http-caching-etags
- **issue-13.8** api-versioning-policy
- **issue-13.9** response-shape-linter

### 4.6 OPSX SDD (la plataforma documentándose a sí misma) (sem 37)

- **issue-04.1** requirements-crud
- **issue-04.2** user-stories-gherkin
- **issue-04.3** specs-designs
- **issue-04.4** tasks-verification
- **issue-04.5** traceability
- **issue-04.6** s3-storage (adjuntos)

### 4.7 Data lifecycle (sem 38)

- **issue-23.1** legacy-import (Notion, Obsidian, MD, JSON)
- **issue-23.2** soft-delete-restore (papelera uniforme + TTL purge)
- **issue-23.3** gdpr-export

### 4.8 Hardening DB final (sem 39-40)

- **issue-25.4** schema-drift (cron daily)
- **issue-25.7** pgaudit-db-level
- **issue-25.9** read-replicas-routing
- **issue-25.10** db-secrets-rotation
- **issue-25.11** anonymization-staging
- **issue-25.12** locks-vacuum-monitoring

**Exit criteria fase 4:**
- [ ] `helm install domain/domain-mcp` levanta cluster prod-ready en AWS/GCP en <30min
- [ ] 3 SDKs publicados (PyPI, npm, proxy.golang.org) con CI de release
- [ ] API completamente versionada con Sunset headers, OpenAPI spec
- [ ] Restore drill mensual automatizado verde
- [ ] OPSX permite documentar nuevas features dentro de Domain (dogfooding meta)
- [ ] Cliente puede exportar todos sus datos GDPR-compliant
- [ ] Cobertura tests >70% global, >80% en service+domain
- [ ] Performance benchmarks baseline establecidos

**Hito marketing:** GA público v1.0, anuncio en redes, blog post, docs.domain.sh.

**Riesgos:**
- Helm chart bugs en producción de clientes: drill exhaustivo en kind
- SDK API contract drift: response shape linter (issue-13.9) crítico
- Data lifecycle: GDPR export en cuentas grandes puede ser lento → background async

---

## Fase 5 — Escala y robustez v1.x (8-10 semanas)

**Objetivo:** soportar 1000+ orgs, 10M+ observations, multi-pod horizontalmente, durable execution. Production-grade serio.

### 5.1 Horizontal scalability (sem 41-44)

- **issue-26.1** stateless-invariant (linter)
- **issue-26.2** leader-election-crons
- **issue-26.3** distributed-locks
- **issue-26.4** graceful-shutdown
- **issue-26.5** circuit-breaker-llm
- **issue-26.6** backpressure-queue
- **issue-26.7** cache-invalidation-patterns

### 5.2 Flow durability + advanced (sem 45-48)

- **issue-09.6** durable-execution (checkpointing + recovery)
- **issue-09.7** workflow-versioning
- **issue-09.8** external-signals
- **issue-09.9** saga-compensation
- **issue-09.10** step-heartbeats
- **issue-09.11** reproducibility-snapshots
- **issue-09.12** dry-run-plan-mode

### 5.3 Multi-agent serio (sem 49-50)

- **issue-08.6** multi-agent-supervisor
- **issue-08.7** agent-handoff
- **issue-08.8** agent-parallel-fanout
- **issue-08.9** agent-hierarchical-context

### 5.4 Runners + outbound (sem 51-52)

- **issue-11.1** sandbox-execution (Docker)
- **issue-11.2** selfhosted-runner
- **issue-11.3** execution-streaming
- **issue-10.4** outbound-webhooks (HMAC + retry + CB + SSRF)

**Exit criteria fase 5:**
- [ ] 10 pods Domain handle 1000 req/s sostenido sin race conditions
- [ ] Flow de 30+ minutos sobrevive restart de pod (durable execution)
- [ ] Multi-agent supervisor coordina 5 sub-agents en paralelo con merge strategy
- [ ] Outbound webhooks delivery >99% éxito con retry policy
- [ ] Self-hosted runner permite cliente correr Domain workers en su infra
- [ ] OpenAI outage 30min → Domain sigue funcionando con fallback Anthropic + cache

**Hito marketing:** v1.5 "Built for scale", caso de éxito con cliente que procesa M+ runs/mes.

---

## Fase 6 — Diferenciación v2.0 (10-12 semanas)

**Objetivo:** features que diferencian de competencia. Custom roles enterprise, vertical performance, web UI minimal, RFCs adicionales para gaps específicos.

### 6.1 Enterprise auth (sem 53-54)

- **issue-02.8** custom-roles-permissions (fine-grained RBAC)

### 6.2 Vertical performance (sem 55-56)

- **issue-27.1** pprof-debug-endpoints
- **issue-27.2** gomaxprocs-gomemlimit
- **issue-27.3** hot-reload-config
- **issue-27.4** feature-benchmarks (regression check en CI)

### 6.3 Web UI mínima (sem 57-60)

- **issue-16.1** web-dashboard (read-only stats)
- **issue-16.2** web-run-visualization (SSE real-time)
- **issue-16.4** web-admin-skills
- **issue-16.5** web-admin-memories

> Nota: issue-16.3 flow-editor (editor visual DAG) postpuesto post-v2.0.

### 6.4 Marketplace / advanced (sem 61-64)

- (HUs nuevas a definir en RFCs cuando aplique)
- Skill marketplace público
- Multi-region deployment
- Plugin system Go/WASM
- Time-travel debugging
- A/B testing de prompts
- AGENT TEMPLATES marketplace community
- i18n base
- Mobile responsive web UI

**Exit criteria fase 6:**
- [ ] Custom roles permite Enterprise definir auditor con permisos read-only sobre observations
- [ ] pprof endpoints en producción permite SRE debug sin restart
- [ ] Web UI permite admin tareas básicas sin abrir CLI
- [ ] Performance benchmarks suite estable, regression detectada en PR

**Hito marketing:** v2.0 Enterprise + Marketplace.

---

## Dependencias entre fases (mermaid)

```
F0 (dev env)
  ↓
F1 (foundation) ──┐
  ↓               │
F2 (MVP alpha)    │
  ↓               │
F3 (beta) ←───────┘ (depende de RBAC, audit, secrets, obs de F1)
  ↓
F4 (v1.0 GA) ───┐
  ↓             │
F5 (scale)      │
  ↓             │
F6 (v2.0) ←─────┘
```

**Crítico:** las HUs de **issue-26 horizontal scalability** son técnicamente útiles desde fase 1 (stateless invariant, graceful shutdown), pero se postponen a fase 5 cuando la presión de carga lo justifica. Si en fase 2-4 detectamos un caso real de carga: subir issue-26.x al sprint en cuestión.

---

## Decisiones de scope a confirmar antes de empezar

| Decisión | Implicación |
|----------|-------------|
| Confirmar tech: Go 1.22+, Postgres 16, pgx v5, mcp-go | bloqueo si cambia |
| Confirmar deploy target inicial: K8s o solo Docker? | K8s afecta REQ-24 prioridad |
| ¿GA v1.0 multi-tenant o single-tenant? | multi-tenant agrega complejidad RLS |
| Pricing tiers definidos antes de fase 3 | bloquea issue-21.3 plans seed |
| Stripe account live antes de fase 3 | bloquea issue-21.4 |
| Cert para emails (DKIM, SPF) prod | bloquea issue-20.2 prod |

---

## Métricas de salud por fase

Tracking continuo (Grafana dashboard auto-generado):

| métrica | F1 target | F2 target | F4 target | F5 target |
|---------|-----------|-----------|-----------|-----------|
| p99 latency `/api/v1/*` | <500ms | <300ms | <200ms | <200ms |
| Error rate | <1% | <0.5% | <0.1% | <0.1% |
| Test coverage global | >50% | >65% | >75% | >80% |
| Time-to-deploy (commit → prod) | n/a | n/a | <20min | <15min |
| Cost per agent_run (USD) | n/a | medido | <$0.10 avg | <$0.05 avg |
| Migration safety (squawk linter) | 100% pass | 100% pass | 100% pass | 100% pass |
| Schema drift incidents | n/a | n/a | 0 | 0 |
| Backup drill success rate | n/a | n/a | 100% | 100% |

---

## Riesgos transversales y mitigaciones

| riesgo | probabilidad | impacto | mitigación |
|--------|--------------|---------|------------|
| LLM cost runaway en dogfooding | alta | medio | cap manual desde fase 2; issue-21.3 en fase 3 |
| RLS performance overhead | media | alto | bench en fase 1, ajustar si >10% |
| OpenAI/Anthropic outage en demos | media | medio | issue-26.5 fallback desde fase 5; demo con Ollama offline |
| GDPR/compliance review tarde | media | alto | revisar legal en fase 3, ajustar fase 4 si necesario |
| Skill marketplace abuse | baja | alto | curated only en v2.0; community submit con review |
| pgvector limit por dimension | baja | medio | docs declaran 1536 (OpenAI ada-002), upgradable |
| Costo cloud durante fase 5 (escala test) | media | medio | load tests con sandbox accounts dedicado |
| Lock-in dependency (Stripe, OpenAI) | media | alto | LLM ya abstraído (REQ-06); Stripe via issue-21.4 con webhook contract estable |
| Skills de community injection | media | alto | Tooling-side sandbox (issue-11.1) + skill review |
| Backup encryption key loss | baja | crítico | KMS-managed + rotation documentada en issue-18.1 |

---

## Equipos sugeridos por fase

Equipo mínimo: **3 devs senior backend** + **1 SRE/DevOps** + **1 product/QA**.

Equipo óptimo (acelera fases 4-6): + **1 frontend** (para Web UI fase 6) + **1 dev relations** (SDKs, docs) + **1 security engineer** (hardening review).

**Especialización por fase:**
- F0-F1: backend Go heavy, SRE para infra
- F2-F3: backend + algún UI (admin minimal)
- F4: + DevRel, SDKs, docs
- F5: backend distributed systems
- F6: + frontend para Web UI

---

## Cómo navegar este roadmap

1. **PM**: usa la tabla "Visión general" para reporting; ajusta fechas con velocity real medida
2. **Dev**: cada HU tiene `hu.md` con Gherkin scenarios y `tasks.md` con bullets atómicas
3. **SRE**: foco fases 1, 4, 5 (REQs 17, 18, 19, 24, 25, 26, 27)
4. **Security**: review continuo de fase 1 + fase 4 (audit_log, RLS, secrets rotation)
5. **Product**: validar exit criteria de cada fase con clientes design partners

---

## Política de updates al roadmap

- Cada fin de fase: review con stakeholders, ajustes a próxima
- Slips >2 semanas en fase: revisar scope (cortar HUs, no extender deadline)
- Nuevas HUs detectadas: agregar a backlog + propuesta de fase
- Cambios de fase target: requieren approval del owner del REQ

---

## Estado del backlog

Total REQs: **27**
Total HUs: **148**
Estado: 100% `proposed` (ninguna implementada aún)

Próximo paso recomendado: **kickoff de Fase 0** (1 semana) y armar squad para Fase 1.
