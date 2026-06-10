# Design: issue-01.9-personas-catalog

## Schema

```sql
CREATE TABLE personas (
  slug VARCHAR(50) PRIMARY KEY,
  name VARCHAR(100) NOT NULL,
  short_description TEXT NOT NULL,
  
  -- Demographics
  demographics JSONB NOT NULL DEFAULT '{}',
  -- {role_archetype, experience_years, team_size_typical, org_size_typical, ...}
  
  -- Goals / Pain Points / Metrics (arrays)
  goals TEXT[] NOT NULL DEFAULT '{}',
  pain_points TEXT[] NOT NULL DEFAULT '{}',
  success_metrics TEXT[] NOT NULL DEFAULT '{}',
  
  -- Tactical
  touchpoints TEXT[] NOT NULL DEFAULT '{}',
  -- e.g. ['mcp', 'cli', 'api', 'web-ui', 'sdk-python']
  
  typical_rbac VARCHAR(50),
  -- Reference to roles built-in (issue-02.2) o custom (issue-02.8)
  
  permissions_typical TEXT[] NOT NULL DEFAULT '{}',
  -- e.g. ['observations:read|write', 'agents:run']
  
  -- Relations
  anti_personas TEXT[] DEFAULT '{}',
  -- "esta persona NO es X, NO es Y"
  
  related_personas TEXT[] DEFAULT '{}',
  -- e.g. ['integrator', 'data-scientist']
  
  -- Body fallback (full markdown)
  body_md TEXT,
  body_md_tsv TSVECTOR GENERATED ALWAYS AS (to_tsvector('spanish', coalesce(body_md, ''))) STORED,
  
  -- Seed management (issue-01.7)
  seed_managed BOOLEAN DEFAULT false,
  seed_version INT,
  is_user_modified BOOLEAN DEFAULT false,
  updated_at_from_seed TIMESTAMPTZ,
  
  -- Versioning
  version INT NOT NULL DEFAULT 1,
  
  -- Audit
  organization_id UUID REFERENCES organizations(id),
  -- NULL = persona global (built-in); set = persona custom de org Enterprise
  
  created_by UUID REFERENCES users(id),
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX ON personas USING GIN (body_md_tsv);
CREATE INDEX ON personas (organization_id) WHERE organization_id IS NOT NULL;

-- Cross-reference tabla (linter alimenta esta tabla cuando procesa hu.md)
CREATE TABLE hu_personas (
  hu_slug VARCHAR(100) NOT NULL,
  persona_slug VARCHAR(50) NOT NULL REFERENCES personas(slug),
  PRIMARY KEY (hu_slug, persona_slug)
);
CREATE INDEX ON hu_personas (persona_slug);
```

## Seed YAML format

```yaml
# seeds/personas/dx-engineer.yaml
slug: dx-engineer
name: "DX Engineer"
short_description: "Desarrollador integrando Domain en su flujo diario via Claude/Cursor/Cline"
demographics:
  role_archetype: "Senior backend engineer (también frontend, full-stack)"
  experience_years: "5-15"
  team_size_typical: "3-10"
  org_size_typical: "10-200"
  technical_depth: "deep"
goals:
  - "Mantener contexto entre sesiones IA sin re-explicar"
  - "Recuperar decisiones arquitectónicas previas rápido"
  - "Acceso fluido a observations/knowledge desde el editor"
  - "Reducir context-switching entre Notion, scratchpad, chat history"
pain_points:
  - "Pierde contexto en cada conversación LLM nueva"
  - "Notas técnicas dispersas en 5 lugares"
  - "Re-explicar convenciones del proyecto al agente cada sesión"
  - "Comprar conocimiento del equipo cuando alguien se va"
success_metrics:
  - "Time to context recovery: <30s en proyecto activo"
  - "Veces que repite explicación al agente: 0"
  - "Skills/agents reutilizados across sessions"
touchpoints:
  - mcp        # principal
  - cli
  - api
typical_rbac: member
permissions_typical:
  - "observations:read|write en sus projects"
  - "sessions:read|write|end"
  - "prompts:read|write"
  - "skills:execute en su org"
  - "agents:run en su org"
  - "knowledge_docs:read en su org"
anti_personas:
  - "NO es admin: no maneja members ni billing"
  - "NO es operator: no toca infra"
  - "NO es auditor: no es read-only"
related_personas:
  - integrator
  - data-scientist
```

## MCP tools

```go
// domain_persona_get
{
  "name": "domain_persona_get",
  "description": "Get a persona definition by slug. Use this when designing features to understand the target user.",
  "input_schema": {
    "type": "object",
    "properties": {
      "slug": {"type": "string"},
      "structured_only": {"type": "boolean", "default": false}
    },
    "required": ["slug"]
  }
}

// domain_persona_list
{
  "name": "domain_persona_list",
  "description": "List all personas (slug + name + short_description).",
  "input_schema": { "type": "object", "properties": {} }
}

// domain_hus_for_persona (cross-ref query)
{
  "name": "domain_hus_for_persona",
  "description": "List HUs that target a specific persona.",
  "input_schema": {
    "type": "object",
    "properties": { "persona_slug": {"type": "string"} },
    "required": ["persona_slug"]
  }
}
```

## Linter extension

```go
// extiende issue-25.13 schema-conventions-linter con:
// - parse hu.md header
// - require **Persona:** field
// - validate slugs exist en personas tabla (o seed YAML al boot)
// - allow multi-persona: **Persona:** dx-engineer, integrator
```

## CLI

```bash
domain personas list
domain personas get dx-engineer
domain personas export-md --to ./docs/personas.md
domain personas import-md --from ./docs/personas.md
domain personas reindex-hu-cross-refs  # rebuild hu_personas table
```

## Retrofit policy

- `.personas-baseline.json` lista las 148 HUs pre-retrofit
- Linter warning (no error) para esas hasta merge del retrofit completo
- Post-retrofit: linter error obligatorio en todas las HUs nuevas y existentes

## TDD plan

1. CRUD personas + audit
2. Seed 10 personas desde YAML
3. MCP get + list + cross-ref
4. Export md round-trip
5. Import md
6. Linter HU sin field → fail
7. Linter slug inexistente → fail
8. Cross-reference table populated correctly
9. Retrofit script: sample HUs → infers correct persona
10. Sabotaje: HU con slug típico mal escrito ("dx_engineer" en vez de "dx-engineer") → linter falla
