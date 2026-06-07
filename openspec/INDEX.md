# Domain — Índice de REQs por Orden de Implementación

> **IDs estables**: Los `REQ-XX` numbering NO refleja orden de implementación. Este índice ordena los REQs según el roadmap (`docs/roadmap.md`) por fase. Cada REQ tiene `**Fase:** FN` en su `req.md` para identificación rápida.
>
> **Total**: 27 REQs activos / 149 HUs / 6 fases

---

## Fase 0 — Bootstrap dev environment (1 semana)

Pre-requisito de todo. Solo infra dev, sin código de producto.

| orden | REQ | HUs core | propósito |
|-------|-----|----------|-----------|
| 1 | **REQ-01** core-platform | HU-01.6, HU-01.2, HU-01.3 | dev env docker-compose + config + health |

---

## Fase 1 — Foundation técnica (6-8 semanas)

Schema completo, auth, observability, seeds, hardening DB. Sin features producto user-facing.

| orden | REQ | HUs core | propósito |
|-------|-----|----------|-----------|
| 1 | **REQ-01** core-platform | HU-01.1, HU-01.4, HU-01.5, HU-01.7, HU-01.8, HU-01.9 | schema + seeders + policies + personas |
| 2 | **REQ-25** db-tooling-hardening | HU-25.3, HU-25.13, HU-25.6, HU-25.5, HU-25.8, HU-25.1, HU-25.2 | linters + RLS + roles + timeouts + pgbouncer |
| 3 | **REQ-17** observability | HU-17.1, HU-17.2, HU-17.3 | metrics + traces + logs |
| 4 | **REQ-02** auth-security | HU-02.1 a HU-02.7 | API keys + RBAC + secrets + audit + OTP |
| 5 | **REQ-21** org-billing (parte) | HU-21.1, HU-21.2 | org mgmt + invitations |

---

## Fase 2 — MVP funcional alpha (8-10 semanas)

Features core del producto: memoria + LLM + skills + agents + MCP + CLI.

| orden | REQ | HUs core | propósito |
|-------|-----|----------|-----------|
| 1 | **REQ-03** memory-system | HU-03.1 a HU-03.7 | observations + sessions + prompts + knowledge + search global |
| 2 | **REQ-06** llm-embeddings | HU-06.1, HU-06.2, HU-06.4, HU-06.5, HU-06.6 | provider abstraction + embeddings |
| 3 | **REQ-05** skill-system | HU-05.1 a HU-05.6 | skills + agent-skill contract |
| 4 | **REQ-08** agent-system (core) | HU-08.1, HU-08.2, HU-08.3, HU-08.5 | CRUD + execution + logs + templates |
| 5 | **REQ-12** mcp-server (core) | HU-12.1, HU-12.2, HU-12.3, HU-12.5 | MCP stdio + memory tools + agent tools + setup |
| 6 | **REQ-13** http-api (base) | HU-13.1, HU-13.2, HU-13.3 | CRUD endpoints + auth middleware + pagination |
| 7 | **REQ-14** cli | HU-14.1, HU-14.2, HU-14.3 | comandos core + output formats + autocomplete |

---

## Fase 3 — Beta privada (6-8 semanas)

Estabilizar para clientes invitados: flows + cron + cost + billing + notifications + cache.

| orden | REQ | HUs core | propósito |
|-------|-----|----------|-----------|
| 1 | **REQ-09** flow-system (core) | HU-09.1 a HU-09.5 | DAG + 10 step types + state machine + retry + subflows |
| 2 | **REQ-10** cron-triggers | HU-10.1, HU-10.2, HU-10.3 | crons + webhooks inbound + event bus |
| 3 | **REQ-15** cost-observability | HU-15.1, HU-15.2, HU-15.3 | token tracking + cost analytics + alertas |
| 4 | **REQ-21** org-billing (resto) | HU-21.3, HU-21.4 | plans + Stripe |
| 5 | **REQ-20** notifications | HU-20.1, HU-20.2, HU-20.3 | channel abstraction + email + Slack |
| 6 | **REQ-12** mcp-server (advanced) | HU-12.4, HU-12.6 | bidirectional + resilience |
| 7 | **REQ-07** context-cache | HU-07.1 a HU-07.4 | context optimizer + stitching + semantic cache + budget |
| 8 | **REQ-02** auth-security (advanced) | HU-02.8 | custom roles |

---

## Fase 4 — Production-ready v1.0 GA (10-12 semanas)

Backup/DR, CI/CD, deployment K8s, SDKs, API maduro, OPSX, data lifecycle.

| orden | REQ | HUs core | propósito |
|-------|-----|----------|-----------|
| 1 | **REQ-18** backup-dr | HU-18.1, HU-18.2, HU-18.3 | pgBackRest + S3 replica + restore drill |
| 2 | **REQ-19** ci-cd | HU-19.1, HU-19.2, HU-19.3 | CI lint+test + release goreleaser + Docker image |
| 3 | **REQ-24** deployment | HU-24.1, HU-24.2, HU-24.3 | Helm + Kustomize + AWS/GCP/k3s examples |
| 4 | **REQ-22** sdk-clients | HU-22.1, HU-22.2, HU-22.3 | Python + TS + Go SDKs |
| 5 | **REQ-13** http-api (mature) | HU-13.4 a HU-13.9 | idempotency + batch + cursor + ETags + versioning |
| 6 | **REQ-04** opsx-sdd | HU-04.1 a HU-04.6 | REQs + HUs + specs + tasks + trace + S3 |
| 7 | **REQ-23** data-lifecycle | HU-23.1, HU-23.2, HU-23.3 | legacy import + soft-delete + GDPR export |
| 8 | **REQ-25** db-tooling-hardening (resto) | HU-25.4, HU-25.7, HU-25.9, HU-25.10, HU-25.11, HU-25.12 | drift + pgaudit + replicas + rotation + anonymization + monitoring |

---

## Fase 5 — Escala y robustez v1.x (8-10 semanas)

Horizontal scalability + durable execution + multi-agent + runners + outbound webhooks.

| orden | REQ | HUs core | propósito |
|-------|-----|----------|-----------|
| 1 | **REQ-26** horizontal-scalability | HU-26.1 a HU-26.7 | stateless + leader election + locks + shutdown + CB LLM + backpressure + cache invalidation |
| 2 | **REQ-09** flow-system (advanced) | HU-09.6 a HU-09.12 | durable + versioning + signals + saga + heartbeats + snapshots + dry-run |
| 3 | **REQ-08** agent-system (multi-agent) | HU-08.6 a HU-08.9 | supervisor + handoff + parallel fan-out + hierarchical context |
| 4 | **REQ-11** runner-system | HU-11.1, HU-11.2, HU-11.3 | sandbox + self-hosted + streaming |
| 5 | **REQ-10** cron-triggers (advanced) | HU-10.4 | outbound webhooks |

---

## Fase 6 — Diferenciación v2.0 (10-12 semanas)

Vertical performance + Web UI minimal + features avanzados.

| orden | REQ | HUs core | propósito |
|-------|-----|----------|-----------|
| 1 | **REQ-27** vertical-performance | HU-27.1 a HU-27.4 | pprof + GOMAXPROCS + hot-reload + benchmarks |
| 2 | **REQ-16** web-ui | HU-16.1, HU-16.2, HU-16.4, HU-16.5 | dashboard + run viz + admin skills + admin memories |
| 3 | (futuros) | TBD | marketplace, plugin system, time-travel debugging, A/B testing prompts |

---

## Dependencias críticas

```
F0 ─→ F1.REQ-01 (schema base) ─┬─→ F1.REQ-25 (hardening DB) ─→ F1.REQ-02 (auth) ─→ F1.REQ-21 (org base)
                               └─→ F1.REQ-17 (obs base)
F1 ─→ F2.REQ-03 (memory) ─→ F2.REQ-05 (skills) ─→ F2.REQ-08 (agents) ─→ F2.REQ-12 (MCP)
                         └─→ F2.REQ-06 (LLM) ──┘
F2 ─→ F2.REQ-13 (HTTP) ─→ F2.REQ-14 (CLI)
F2 ─→ F3 (flows + cron + cost + billing + notif)
F3 ─→ F4 (BCP + CI/CD + Deploy + SDKs + API mature + OPSX)
F4 ─→ F5 (escala + multi-agent + runners)
F5 ─→ F6 (vertical perf + Web UI)
```

## Conteo por fase

| fase | REQs involucrados (parcial o total) | HUs nuevas a implementar |
|------|--------------------------------------|---------------------------|
| F0 | 1 | 3 |
| F1 | 5 | 22 |
| F2 | 7 | 28 |
| F3 | 8 | 24 |
| F4 | 8 | 32 |
| F5 | 5 | 22 |
| F6 | 3 | 17 |
| **Total** | 27 | **148** |

(HU-01.9 personas-catalog cuenta como Fase 1; 149 HUs totales)

## Acción

Para empezar la próxima HU, mirar la primera fase con HUs sin implementar y elegir la primera HU del orden listado arriba. Ver `state.yaml` per HU para status actual.
