# Domain — Catálogo de Personas

> Este documento es **generado desde la BD** (`personas` table) vía `domain personas export-md`. NO editar a mano salvo en seeds YAML (`seeds/personas/*.yaml`).
>
> **Última generación:** 2026-06-07
> **Source of truth:** BD `personas` table (HU-01.9)

## Concepto

Persona = **actor** que interactúa con Domain. NO es estrictamente "user de la app" — incluye roles externos (auditor, regulador), técnicos (SRE), y stakeholders no-user. Cada HU referencia 1+ personas en su header `**Persona:** slug` para explicitar a quién está dirigida.

## Tabla resumen

| slug | nombre | rol primario | RBAC default | touchpoints principales |
|------|--------|--------------|--------------|-------------------------|
| `dx-engineer` | DX Engineer | Developer usando Domain día a día | member | MCP, CLI, API |
| `platform-engineer` | Platform Engineer / SRE | Opera infra Domain de su instalación | admin | CLI, helm, métricas |
| `security-officer` | Security Officer / Compliance | Audit, threat model, GDPR, secrets | admin (audit-scoped) | API, web admin |
| `org-owner` | Organization Owner | Líder de su org en Domain | owner | Web admin, API, CLI |
| `org-admin` | Organization Admin | Admin operativo de org | admin | Web admin, API, CLI |
| `org-member` | Organization Member | Miembro individual de org | member | MCP, web, API |
| `platform-admin` | Platform Admin (Domain SaaS) | Opera el servicio Domain entero | platform_admin | Endpoints admin, CLI privileged |
| `auditor` | Auditor / Compliance Reviewer | Read-only para review | viewer (audit-scoped) | API + reports |
| `integrator` | Integrator / Builder | Construye producto encima de Domain | varía (member o admin de su org) | SDK, API |
| `data-scientist` | Data Scientist / ML Engineer | Analiza datos Domain con Python | member | SDK Python, API |

---

## 1. `dx-engineer` — DX Engineer

**Tagline:** *"Quiero que mi agente IA recuerde lo que ya le expliqué."*

### Demografía
- **Rol:** Senior backend engineer (también full-stack)
- **Experiencia:** 5-15 años
- **Equipo típico:** 3-10 personas
- **Org típica:** 10-200 personas
- **Profundidad técnica:** deep

### Goals
- Mantener contexto entre sesiones con Claude/Cursor/Cline sin re-explicar
- Recuperar decisiones arquitectónicas previas rápido
- Acceso fluido a observations/knowledge desde el editor (vía MCP)
- Reducir context-switching entre Notion, scratchpad, chat history

### Pain points sin Domain
- Pierde contexto en cada conversación LLM nueva
- Notas técnicas dispersas en 5 lugares
- Re-explicar convenciones del proyecto al agente cada sesión
- Conocimiento del equipo se pierde cuando alguien se va

### Cómo Domain le ayuda
- MCP tools `domain_mem_save/search/context` integrados con su agente
- Skills reusables encapsulan lógica que el agente invoca
- Knowledge docs con RAG-ready embeddings
- Cross-project search global (HU-03.7)

### Success metrics
- Time to context recovery: <30s en proyecto activo
- Veces que repite explicación al agente: 0
- Skills/agents reutilizados across sessions

### Touchpoints
- **MCP server** (Claude Code, Cursor, Cline) — primario
- **CLI** `domain`
- **HTTP API** ocasional

### RBAC default: `member`

Permisos típicos:
- `observations:read|write` en sus projects
- `sessions:read|write|end`
- `prompts:read|write`
- `skills:execute` en su org
- `agents:run` en su org
- `knowledge_docs:read` en su org

### NO es (anti-persona)
- NO es admin: no maneja members ni billing
- NO es operator: no toca infra
- NO es auditor: no es read-only

### Personas relacionadas
- `integrator` — puede serlo simultáneamente si construye producto encima
- `data-scientist` — caso especial de DX más analítico

---

## 2. `platform-engineer` — Platform Engineer / SRE

**Tagline:** *"Necesito que Domain corra robusto, observable y escalable en mi infra."*

### Demografía
- **Rol:** SRE, Platform Engineer, DevOps
- **Experiencia:** 7-20 años
- **Equipo típico:** 2-8 SRE en org de 50-500
- **Profundidad infra:** deep (K8s, Postgres, observability)

### Goals
- Deploy Domain en su cluster K8s con Helm + valores apropiados
- SLOs cumplidos: 99.9% uptime, p99 <500ms
- Visibilidad métricas + traces + logs
- Backups + restore drill mensual verde
- Cero downtime en upgrades

### Pain points sin Domain bien hardened
- "Otra DB que mantener" — quiere tooling estándar (PgBouncer, pgaudit, backups con pgBackRest)
- Productos AI vendor con observability débil
- Migrations peligrosas que tumban prod

### Cómo Domain le ayuda
- Helm chart oficial con HPA + PDB + NetworkPolicy (HU-24.1)
- Métricas Prometheus + traces OTel + logs JSON (REQ-17)
- Backups pgBackRest PITR + drill mensual (REQ-18)
- Migration linter bloquea PRs peligrosos (HU-25.3)
- Read replicas + connection pooler (HU-25.1, HU-25.9)
- Graceful shutdown + leader election (HU-26.2, HU-26.4)

### Success metrics
- Uptime: 99.9%+ mensual
- MTTR incident: <30 min
- Backup drill success rate: 100%
- Deploy frequency: daily sin incidentes

### Touchpoints
- **`kubectl` + `helm`** — primario
- **Grafana** dashboards + alerts
- **CLI domain** (admin operations, backup, restore)
- **Endpoints admin** REST

### RBAC default: `admin` o `platform_admin` (cuando es SaaS managed)

Permisos típicos:
- Todo lo operacional: configs, runtime_configs, system_crons
- Read sobre datos de orgs (pero NO read sobre content sensible salvo auth explícita)
- Trigger backups/restores
- View métricas + logs centralizados

### NO es
- NO desarrolla features de producto
- NO es security officer (aunque colabora)
- NO debería tener acceso routinario a data de clientes

### Personas relacionadas
- `security-officer` (colabora en hardening, audit)
- `platform-admin` (SaaS multi-tenant variante)

---

## 3. `security-officer` — Security Officer / Compliance

**Tagline:** *"Cada acceso a datos sensibles debe ser auditable y justificable."*

### Demografía
- **Rol:** Security engineer, compliance officer, CISO en SMB
- **Experiencia:** 8-20 años
- **Equipo típico:** 1-5 security
- **Profundidad:** mix técnico + regulatorio (SOC2, ISO27001, GDPR, HIPAA)

### Goals
- Audit trail completo e inmutable de toda operación crítica
- RBAC fine-grained y revisable
- Secrets gestionados con rotation policy
- Compliance reports generables on-demand
- GDPR rights del usuario (export, delete) automatizables

### Pain points sin Domain hardened
- Audit logs incompletos o mutables
- Secrets en plaintext en env vars
- "Quién hizo qué cuándo" no recuperable
- GDPR export: ticket manual de varios días

### Cómo Domain le ayuda
- audit_log inmutable + activity_log user-facing (HU-02.4/6)
- pgaudit a nivel DB (HU-25.7)
- AES-256-GCM encryption de secrets + rotation (HU-02.3)
- RBAC built-in + custom roles (HU-02.2, HU-02.8)
- RLS selectivo en tablas sensibles (HU-25.5)
- GDPR export automatizado (HU-23.3)
- Password rotation zero-downtime (HU-25.10)

### Success metrics
- Audit trail completeness: 100%
- Time to compliance report: <1 día
- Secrets in plaintext: 0
- Cross-org leak incidents: 0

### Touchpoints
- **Endpoints admin** (audit, compliance reports)
- **Web admin** vista audit_log (cuando exista)
- **SQL ad-hoc** sobre audit DB
- **Reports automatizados** vía notifications

### RBAC default: `admin` con scope audit-only en algunas implementaciones, `platform_admin` para acceso completo

Permisos típicos:
- `audit_log:read`
- `activity_log:read`
- `users:read`, `organizations:read`
- `secrets:read|write` (rotate)
- `custom_roles:write`
- `runtime_configs:read`

### NO es
- NO desarrolla features
- NO opera infra (delega a platform-engineer)

### Personas relacionadas
- `platform-engineer` (colabora en hardening)
- `auditor` (entrega data al auditor; pero security-officer está dentro del equipo, auditor es externo)

---

## 4. `org-owner` — Organization Owner

**Tagline:** *"Mi org en Domain es mi territorio: yo decido members, plan, billing."*

### Demografía
- **Rol:** Founder, CTO, Engineering Manager, líder de squad
- **Experiencia:** variable
- **Org size:** 5-200 personas (responde por una "team" en Domain)

### Goals
- Onboardear su equipo en Domain (invitaciones)
- Elegir plan apropiado y gestionar billing
- Configurar políticas de su org (default model, timezone)
- Transferir ownership cuando se va (sin perder data)

### Pain points
- Setup inicial multi-tenant complicado
- Cambio de plan ambiguo (¿qué pierdo?)
- Transfer ownership manual / con soporte

### Cómo Domain le ayuda
- CRUD org self-service (HU-21.1)
- Invitaciones email con token + Google OAuth auto-accept (HU-21.2)
- Stripe Checkout + Customer Portal (HU-21.4)
- Transfer ownership con re-auth (HU-21.1)
- Plans + cuotas visibles + alertas 80% (HU-21.3)

### Success metrics
- Setup org → primer agente corriendo: <10 min
- Invitations aceptadas en <24h: >90%
- Plan upgrade friction: 0 (Stripe Checkout self-service)

### Touchpoints
- **Web admin** (cuando exista) — primario para billing
- **HTTP API** + CLI para automatización
- **Email** para invitations / billing receipts

### RBAC default: `owner` (único per org)

Permisos típicos:
- ALL on su org
- Transfer ownership
- Delete org (con confirmación)
- Manage billing + payment methods

### NO es
- NO maneja infra global (eso es platform-engineer/admin)
- NO tiene acceso cross-org

### Personas relacionadas
- `org-admin` (delegate operacional)
- `org-member` (usuarios que invita)

---

## 5. `org-admin` — Organization Admin

**Tagline:** *"Hago que mi org funcione día a día: members, projects, skills, agents."*

### Demografía
- **Rol:** Tech Lead, Senior Engineer designado, EM operacional
- **Experiencia:** 5-15 años

### Goals
- Gestionar members (invitar, asignar roles, revocar)
- Configurar projects + templates (HU-01.4)
- Curar skills/agents de la org
- Monitorear uso vs plan + alertas

### Pain points
- Member onboarding repetitivo
- Skills mal-mantenidas se acumulan
- No visibilidad de cost por team/project

### Cómo Domain le ayuda
- Bulk invitations (HU-21.2)
- Project templates (HU-01.4) con skills/agents/flows preconfigurados
- Cost analytics por project/user (HU-15.2)
- Custom roles para granularidad (HU-02.8)

### Success metrics
- Time member invitation → activo: <1h
- Skills aprobadas / total en registry: >80%
- Cost per project visible y trackeable

### Touchpoints
- **Web admin**, **API**, **CLI**

### RBAC default: `admin`

Permisos típicos:
- Manage members + roles (excepto owner)
- CRUD projects, skills, agents, flows, prompts, knowledge_docs
- Read billing + plans (no write)

### NO es
- NO maneja billing (eso es owner)
- NO transfiere ownership

---

## 6. `org-member` — Organization Member

**Tagline:** *"Uso Domain en mi trabajo diario sin pensar en admin stuff."*

### Demografía
- **Rol:** Cualquier persona en una org Domain
- **Experiencia:** variable

### Goals
- Guardar observaciones rápido
- Encontrar contexto previo
- Ejecutar agentes con confianza
- Compartir conocimiento con su equipo

### Pain points
- Onboarding muy técnico al principio
- No saber qué skills/agents existen disponibles

### Cómo Domain le ayuda
- MCP integration setup automático (HU-12.5)
- Auto-skill engine recomienda skills relevantes (HU-05.4)
- Búsqueda global con FTS+vector (HU-03.7)
- Templates predefinidos (HU-08.5, HU-01.4)

### Success metrics
- Time first useful interaction: <1 hora post-invitation
- Repeated use weekly: >80%

### Touchpoints
- **MCP** (Claude/Cursor/Cline) — primario
- **Web minimal** (auth + ver datos propios)
- **CLI ocasional**

### RBAC default: `member`

Permisos típicos:
- CRUD sobre own observations/sessions/prompts
- Read shared knowledge_docs
- Execute skills/agents que existen
- NO crea agents/skills nuevos por default (puede el admin habilitarlo)

### NO es
- NO admin
- NO maneja settings org

---

## 7. `platform-admin` — Platform Admin (Domain SaaS)

**Tagline:** *"Yo opero el servicio Domain entero — todos los tenants."*

> Solo aplica si Domain corre en modo SaaS multi-tenant. En self-hosted single-org, este rol no aplica.

### Demografía
- **Rol:** SRE platform team del SaaS provider
- **Experiencia:** 10+ años

### Goals
- Mantener Domain SaaS up para 100s/1000s de orgs
- Onboarding de nuevos clientes (Enterprise)
- Cross-tenant analytics (revenue, MAU, top features)
- Incident response across orgs

### Pain points (sin tooling)
- Multi-tenant admin típicamente requiere queries SQL ad-hoc

### Cómo Domain le ayuda
- Endpoints `/admin/*` con RBAC platform_admin
- Cross-org analytics aggregated
- Audit trail global
- Schema drift detection (HU-25.4)

### Success metrics
- MTTR cross-tenant incident: <1h
- New Enterprise org onboarding: <1 día

### Touchpoints
- **Endpoints `/admin/*`**
- **`domain` CLI** con privileges
- **Direct DB access** (último recurso, audited)

### RBAC default: `platform_admin`

Permisos típicos:
- Cross-org read
- Custom roles + plans management
- Trigger backups, drills, password rotations
- Schema drift checks

### NO es
- NO es member de las orgs que administra (no debería tener "data access" routinario)

---

## 8. `auditor` — Auditor / Compliance Reviewer

**Tagline:** *"Necesito evidencia de que cumplen SOC2/GDPR sin tocar el sistema productivo."*

### Demografía
- **Rol:** Auditor externo (Big 4, boutique compliance firm) o interno
- **Experiencia:** 5-20 años en compliance

### Goals
- Read-only access para review
- Audit trail completo y exportable
- Reports estandarizados (SOC2 controls, GDPR record)

### Pain points
- Audit logs incompletos o inmodificables
- Acceso ad-hoc sin trazabilidad
- Reports hand-built por equipo cliente

### Cómo Domain le ayuda
- Custom role audit-only via HU-02.8
- audit_log + pgaudit ambos visibles
- Reports compliance auto-generables
- GDPR export del usuario individual

### Success metrics
- Audit period sin findings críticos
- Time to deliver evidence: <1 semana

### Touchpoints
- **API read-only** con custom role
- **Reports export** (CSV, PDF)

### RBAC default: custom role `auditor`, scope-limited

Permisos típicos:
- `audit_log:read` ALL orgs (si está fuera del cliente) o solo su org
- `activity_log:read`
- `users:read` (sin PII full salvo justificable)
- Reports compliance generation

### NO es
- NO escribe nada
- NO accede contenido user (observations content) salvo con autorización

---

## 9. `integrator` — Integrator / Builder

**Tagline:** *"Construyo mi producto encima de Domain — necesito API estable y SDKs serios."*

### Demografía
- **Rol:** Founder técnico, lead engineer en producto que integra Domain
- **Experiencia:** 5-20 años

### Goals
- API estable con SemVer + deprecation policy
- SDKs idiomáticas en Python/TS/Go
- Webhooks outbound para reaccionar a eventos
- Custom OAuth flow para sus end-users (futuro)

### Pain points
- APIs que rompen sin warning
- SDKs incompletos
- Polling en lugar de webhooks

### Cómo Domain le ayuda
- API versioning policy (HU-13.8) con Sunset headers 12 meses
- 3 SDKs oficiales (REQ-22)
- Outbound webhooks (HU-10.4) con HMAC + retry
- Idempotency-Key support (HU-13.4)
- Bulk batch endpoints (HU-13.5)
- ETags + optimistic concurrency (HU-13.7)

### Success metrics
- API breaking changes / año: 0 sin major version bump
- Time to first API call (SDK): <15 min
- Webhook delivery >99%

### Touchpoints
- **SDK** (Python/TS/Go)
- **HTTP API** direct
- **Outbound webhooks** receiver

### RBAC default: variable — su producto puede ser member o admin de orgs cliente

### NO es
- NO opera Domain (consume el API)
- NO necesariamente UI user de Domain

### Personas relacionadas
- `dx-engineer` (puede ser ambas a la vez)

---

## 10. `data-scientist` — Data Scientist / ML Engineer

**Tagline:** *"Quiero analizar runs/costos/skills patterns para mejorar nuestros prompts y agentes."*

### Demografía
- **Rol:** DS, MLE, AI researcher
- **Experiencia:** 3-15 años

### Goals
- Acceder a datos de runs (tokens, costo, latency) para análisis
- Bulk export para entrenar/evaluar
- Notebooks Jupyter con SDK Python

### Pain points
- Datos en formato no exportable
- API que no soporta paginación eficiente

### Cómo Domain le ayuda
- SDK Python con async iterator pagination (HU-22.1)
- Bulk endpoints (HU-13.5)
- Cost analytics granular (HU-15.2)
- GDPR-grade export (HU-23.3)
- Cross-project search (HU-03.7) para análisis cualitativos

### Success metrics
- Time to first analysis: <30 min
- Datasets reproducibles: 100%

### Touchpoints
- **SDK Python** — primario
- **Jupyter / Colab**
- **Bulk export API**

### RBAC default: `member`, posiblemente custom role analytics-read

### NO es
- NO escribe data normalmente (read-heavy)

### Personas relacionadas
- `dx-engineer` (variante analítica)

---

## Convención de uso en HUs

Cada `hu.md` declara en su header:

```markdown
# HU-XX.Y-name

**Origen:** `REQ-XX-name`
**Persona:** dx-engineer
...
```

Multi-persona OK:

```markdown
**Persona:** dx-engineer, integrator
```

Linter (HU-25.13 extendido) valida:
- Field `**Persona:**` presente
- Slug existe en tabla `personas`
- Si listed en `.personas-baseline.json` (legacy retrofit pending), warning en lugar de error

## Comandos relacionados

```bash
domain personas list
domain personas get dx-engineer
domain personas export-md --to ./docs/personas.md
domain personas import-md --from ./docs/personas.md
domain personas reindex-hu-cross-refs

# Listar HUs por persona
domain personas hus dx-engineer
```

MCP tools para agentes IA:
- `domain_persona_get(slug)`
- `domain_persona_list()`
- `domain_hus_for_persona(slug)`
