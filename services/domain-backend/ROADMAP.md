# Domain — Roadmap a 100%

Plan de entregas por fases para completar el 100% de las HUs del proyecto.
Cada fase deja el producto funcional y testeable.

---

## Fase 0 — Cimientos (Fundación)

**Objetivo:** Base sólida para todo lo demás. Core platform, DB hardened, observabilidad y performance.

| REQ | HUs | Estado actual |
|-----|-----|--------------|
| REQ-01 Core Platform | 01.4 project-templates, 01.7 seeders-system, 01.8 platform-policies | 3 partial |
| REQ-25 DB Hardening | 25.1 a 25.13 (13 HUs) | 12 partial, 1 proposed |
| REQ-17 Observability | 17.1 metrics-prometheus, 17.2 tracing-otel, 17.3 logging ✅ | 2 partial, 1 done |
| REQ-27 Performance | 27.1 pprof, 27.2 gomaxprocs, 27.3 hot-reload, 27.4 benchmarks | 4 partial |

---

## Fase 1 — Auth & Seguridad (Puertas)

**Objetivo:** Todo el perímetro de seguridad — auth, RBAC, audit, rate-limit, OTP.

| REQ | HUs |
|-----|-----|
| REQ-02 Auth & Security | 02.1 API key, 02.2 RBAC, 02.3 secrets encrypt, 02.4 audit log, 02.5 rate-limit/PII, 02.6 activity log, 02.7 OTP passwordless, 02.8 custom roles |

---

## Fase 2 — Memoria & Datos (Cerebro)

**Objetivo:** Capa de almacenamiento inteligente con search, versionado y compliance GDPR.

| REQ | HUs |
|-----|-----|
| REQ-03 Memory System | 03.1 observations CRUD+FTS, 03.2 sessions, 03.3 prompts, 03.4 knowledge docs, 03.5 context timeline, 03.6 dedup/privacy, 03.7 cross-project search |
| REQ-23 Data Lifecycle | 23.1 legacy import, 23.2 soft-delete, 23.3 GDPR export, 23.4 right-to-erasure |

---

## Fase 3 — LLM & AI Engine (Mente)

**Objetivo:** Proveedores LLM, embeddings, caching semántico y modelo de costos.

| REQ | HUs |
|-----|-----|
| REQ-06 LLM/Embeddings | 06.1 provider factory, 06.2 runners, 06.3 Ollama, 06.4 model registry, 06.5 embedding pgvector, 06.6 token count/stream |
| REQ-07 Context Cache | 07.1 context optimizer, 07.2 cross-session stitch, 07.3 semantic cache, 07.4 token budget |

---

## Fase 4 — Agentes (Fuerza de trabajo)

**Objetivo:** Sistema de agentes completo con definiciones, ejecución, logs y orquestación.

| REQ | HUs |
|-----|-----|
| REQ-08 Agent System | 08.1 definitions, 08.2 execution, 08.3 runs/logs, 08.4 multi-agent, 08.5 templates, 08.6 supervisor, 08.7 handoff, 08.8 parallel, 08.9 hierarchical context |

---

## Fase 5 — Flujos & Automatización (Fábrica)

**Objetivo:** Orquestación de trabajo con DAG, state machine, triggers, runners y compensación.

| REQ | HUs |
|-----|-----|
| REQ-09 Flow System | 09.1-09.12 (12 HUs: DAG, steps, state machine, retry, subflows, durable, versioning, signals, saga, heartbeats, snapshots, dry-run) |
| REQ-10 Cron/Triggers | 10.1 cron, 10.2 webhooks, 10.3 event execution, 10.4 outbound webhooks |
| REQ-11 Runner System | 11.1 sandbox, 11.2 self-hosted, 11.3 streaming |

---

## Fase 6 — Skills & MCP (Ecosistema)

**Objetivo:** Skills reutilizables + protocolo MCP robusto.

| REQ | HUs |
|-----|-----|
| REQ-05 Skill System | 05.1 definitions, 05.2 registry/search, 05.3 versioning, 05.4 auto-skill, 05.5 execution, 05.6 agent-skill contract |
| REQ-12 MCP Server | 12.1 core stdio, 12.2 memory tools, 12.3 agent tools, 12.4 bidirectional, 12.5 setup wizard, 12.6 tool resilience |

---

## Fase 7 — APIs & SDKs (Superficie de contacto)

**Objetivo:** Interfaces externas: REST API, CLI y SDKs multi-lenguaje.

| REQ | HUs |
|-----|-----|
| REQ-13 HTTP API | 13.1 CRUD endpoints, 13.2 auth middleware, 13.3 pagination, 13.4 idempotency, 13.5 bulk, 13.6 cursor, 13.7 caching/etags, 13.8 versioning, 13.9 response linter |
| REQ-14 CLI | 14.1 core commands, 14.2 output formats, 14.3 autocomplete/help |
| REQ-22 SDKs | 22.1 Python, 22.2 TypeScript, 22.3 Go |

---

## Fase 8 — UX, Negocio & Consumo

**Objetivo:** Interfaces de usuario, notificaciones, billing y cost analytics.

| REQ | HUs |
|-----|-----|
| REQ-16 Web UI | 16.1 dashboard, 16.2 run visualization, 16.3 flow editor, 16.4 admin skills, 16.5 admin memories |
| REQ-20 Notifications | 20.1 channel abstraction, 20.2 email SMTP, 20.3 Slack |
| REQ-21 Org/Billing | 21.1 org management, 21.2 invitations, 21.3 plans/limits |
| REQ-15 Cost/Observability | 15.1 token tracking, 15.2 cost analytics, 15.3 usage alerts |

---

## Fase 9 — Escalabilidad & Delivery

**Objetivo:** Producción en K8s, escalabilidad horizontal y tooling SDD completo.

| REQ | HUs |
|-----|-----|
| REQ-24 Deployment | 24.1 Helm chart, 24.2 Kustomize, 24.3 K8s examples |
| REQ-26 Scalability | 26.1 stateless, 26.2 leader election, 26.3 distributed locks, 26.4 graceful shutdown, 26.5 circuit breaker, 26.6 backpressure, 26.7 cache invalidation |
| REQ-04 Opsx SDD | 04.1 requirements CRUD, 04.2 gherkin, 04.3 specs/designs, 04.4 tasks, 04.5 traceability, 04.6 S3 storage, 04.7 interactive builder |
| REQ-19 CI/CD | 19.1 lint-test, 19.2 release binary, 19.3 docker image |

---

## Principios de ejecución

1. **SDD First** — toda HU tiene su spec antes de codear
2. **TDD** — Red → Green → Refactor → Sabotaje
3. **Una HU por rama** — feat/REQ-XX-descripcion
4. **Commit por intención** — un tipo por commit (feat/fix/refactor/etc)
5. **Sin build después de cambios** — solo lint + test
6. **Fase n depende de Fase n-1** — no saltarse fases
