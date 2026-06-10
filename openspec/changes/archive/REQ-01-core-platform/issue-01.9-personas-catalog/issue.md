# issue-01.9-personas-catalog

**Origen:** `REQ-01-core-platform`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** mantenedor del proyecto Domain
**Quiero** un catálogo formal de personas (actores) que el sistema sirve, persistido en BD y consultable por agentes IA
**Para** que cada HU explicite a quién está dirigida (no implícito), el agente generador entienda perspectiva, y RBAC + flows se alineen con personas reales

## Aclaración importante

**Persona ≠ user de la app.** Una persona puede ser:
- Rol interno (employee, member de org)
- Rol externo (cliente, partner, integrador, regulador)
- Rol técnico (SRE, security officer)
- Stakeholder no-user (auditor externo que solo recibe reports)

## Las 10 personas core

| slug | nombre | scope |
|------|--------|-------|
| `dx-engineer` | DX Engineer | Developer usando Domain día a día via Claude/Cursor/Cline (MCP), CLI, SDK |
| `platform-engineer` | Platform Engineer / SRE | Operador de la infra Domain (su instalación) |
| `security-officer` | Security Officer / Compliance | Audit, threat model, RBAC, GDPR, secrets |
| `org-owner` | Organization Owner | Fundador/líder de una org Domain: billing, plans, transfer ownership |
| `org-admin` | Organization Admin | Admin operativo de una org: members, projects, skills, agents |
| `org-member` | Organization Member | Usuario individual en una org, usa Domain en su trabajo |
| `platform-admin` | Platform Admin (Domain SaaS) | Admin del servicio Domain entero (multi-tenant SaaS) |
| `auditor` | Auditor | Read-only access para compliance/regulatory review |
| `integrator` | Integrator / Builder | Developer que construye producto encima de Domain via SDK/API |
| `data-scientist` | Data Scientist / ML Engineer | Análisis de memoria/runs/costos vía SDK Python |

## Schema de cada persona

```yaml
slug: dx-engineer
name: "DX Engineer"
short_description: "Desarrollador que usa Domain día a día"
demographics:
  role_archetype: "Senior backend engineer"
  experience_years: "5-15"
  team_size_typical: "3-10"
  org_size_typical: "10-200"
goals:
  - "Mantener contexto entre sesiones con Claude/Cursor sin re-explicar"
  - "Acceder rápido a observaciones/notas técnicas relevantes"
  - "Que el agente IA recuerde decisiones arquitectónicas del proyecto"
pain_points_without_domain:
  - "Pierde contexto cada conversación nueva"
  - "Notas técnicas dispersas (Notion + scratchpad + chat history)"
  - "Tiene que reentrenar al agente en convenciones del proyecto"
success_metrics:
  - "Time to context recovery: <30s"
  - "Veces que repite explicación: 0 dentro de un proyecto activo"
  - "Tareas completadas con asistencia IA / día: 2x baseline"
touchpoints_primarios:
  - "MCP server (Claude Code, Cursor, Cline)"
  - "CLI domain"
  - "HTTP API ocasional"
typical_rbac: member
permissions_typical:
  - "observations:read|write en sus projects"
  - "skills:execute"
  - "agents:run"
  - "knowledge_docs:read"
anti_personas:
  - "NO es admin: no maneja billing, members, plans"
  - "NO es operator: no toca infra"
related_personas:
  - "integrator (puede serlo simultáneamente si construye producto encima)"
  - "data-scientist (caso especial de DX más analítico)"
```

## Criterios de aceptación

### Escenario 1: Tabla y schema

```gherkin
Dado que existe tabla `personas` con shape definido en design.md
Cuando inspecciono
Entonces tiene columnas: slug PK, name, short_description, demographics JSONB, goals TEXT[], pain_points TEXT[], success_metrics TEXT[], touchpoints TEXT[], typical_rbac VARCHAR, permissions_typical TEXT[], anti_personas TEXT[], related_personas TEXT[], created_at, updated_at, seed_managed BOOLEAN, is_user_modified BOOLEAN
```

### Escenario 2: Seed inicial 10 personas

```gherkin
Dado que boot inicial con seeders issue-01.7
Cuando se ejecuta `personas` seeder
Entonces se UPSERT 10 personas desde YAML embebidos `seeds/personas/*.yaml`
Y cada una tiene los 10 campos completos (goals, pains, metrics, etc.)
```

### Escenario 3: MCP tools

```gherkin
Dado que existe MCP tool `domain_persona_get(slug)`
Cuando un agente IA invoca con slug="dx-engineer"
Entonces devuelve el persona completo en JSON
Y respeta resilience issue-12.6 (timeout + cache + degraded)

Dado que existe `domain_persona_list()`
Cuando se invoca
Entonces devuelve array de {slug, name, short_description} sin detalle full
```

### Escenario 4: HU referencia persona

```gherkin
Dado que cada hu.md tiene header field `**Persona:** dx-engineer` (o multiple `dx-engineer, integrator`)
Cuando linter (issue-25.13 extendido) procesa hu.md
Entonces valida que cada slug referenciado existe en `personas` tabla
Y CI fail si slug no existe
Y HUs nuevas sin field `Persona:` → CI fail
```

### Escenario 5: Export markdown

```gherkin
Dado que CLI `domain personas export-md --to ./docs/personas.md`
Cuando se ejecuta
Entonces se regenera doc human-readable desde BD
Y formato Keep a Catalog: tabla resumen + sección por persona
Y idempotente (mismos datos → mismo MD)
```

### Escenario 6: Import markdown

```gherkin
Dado que CLI `domain personas import-md --from ./docs/personas.md`
Cuando se ejecuta
Entonces UPSERT en `personas` (si parser detecta secciones bien formateadas)
Y reporta diff
```

### Escenario 7: Endpoint admin

```gherkin
Dado que admin platform consulta GET /admin/personas
Cuando se procesa
Entonces devuelve lista con RBAC platform_admin
Y POST /admin/personas crea nuevo (custom personas para Enterprise plans futuros)
```

### Escenario 8: Cross-reference HU ↔ persona

```gherkin
Dado que issue-02.7 tiene `Persona: org-member, integrator`
Cuando GET /api/v1/sdd/personas/dx-engineer/hus
Entonces devuelve todas las HUs que referencian esa persona
Y permite review per-persona: "qué tiene Domain para mí"
```

### Escenario 9: Linter retrofit baseline

```gherkin
Dado que existe `.personas-baseline.json` con lista de HUs pre-retrofit
Cuando linter procesa una HU listada en baseline sin field Persona
Entonces warning (no error) — solo nuevas HUs fail
Y al merge de retrofit completo, baseline se vacía y todas requieren field
```

## Análisis breve

- **Qué pide:** tabla `personas` + 10 seedeadas YAML + 2 MCP tools + CLI export/import + linter cross-reference + endpoint admin + retrofit policy
- **Esfuerzo:** M
- **Riesgos:** drift entre BD y MD si import no se corre; personas custom proliferación (cap razonable: max 30 per org)
