# Design: issue-05.1-skill-definitions

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativa | Razón |
|----------|---------------|-------------|-------|
| ORM | sqlx + scans manuales | GORM | Control fino sobre queries + pgvector support |
| Validación JSON Schema | santhosh-tekuri/jsonschema | go-playground/validator | Soportael schema completo de JSON Schema (draft-07) |
| Embedding | Llamada async post-save | Síncrono obligatorio | Graceful degradation si embedding provider falla |
| Slug constraint | Unique compuesto (project_id, slug) | Slug global único | Skills pueden tener mismo nombre en distintos proyectos |
| Tags | TEXT[] con GIN index | Tabla separada tags | Performance en filtros array overlap |

## Alternativas descartadas

- **GORM:** Demasiada magia para queries con pgvector. Preferimos sqlx con queries explícitas.
- **Slug global único:** Rompe el aislamiento por proyecto.
- **Embedding síncrono obligatorio:** El skill no se crearía si el LLM provider falla, rompiendo disponibilidad.

## Diagrama

```
POST /api/skills
  │
  ├─► Validar payload (name, slug, type, parameters schema)
  ├─► Validar slug único (project_id, slug)
  ├─► INSERT en skills (embedding = NULL)
  ├─► Disparar goroutine: generar embedding vía provider
  │     └─► UPDATE skills SET embedding = ? WHERE id = ?
  └─► Responder 201 con skill creado

GET /api/skills?type=prompt&project_id=proj-abc&tags=resumen
  │
  ├─► WHERE type = $1 AND project_id = $2 AND tags && $3
  └─► Responder 200 con paginación

DELETE /api/skills/:id
  │
  ├─► Verificar dependencias en flow_steps, agent_skills
  ├─► Si hay dependencias → 409
  └─► DELETE → 204
```

### Modelo

```sql
CREATE TABLE skills (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    slug        VARCHAR(255) NOT NULL,
    description TEXT,
    type        VARCHAR(50) NOT NULL CHECK (type IN ('prompt','code','api','mcp_tool')),
    content     TEXT NOT NULL,
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    parameters  JSONB,
    return_type JSONB,
    tags        TEXT[] DEFAULT '{}',
    embedding   vector(1536),
    version     INT NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, slug)
);

CREATE INDEX idx_skills_type ON skills(type);
CREATE INDEX idx_skills_project ON skills(project_id);
CREATE INDEX idx_skills_tags ON skills USING GIN(tags);
CREATE INDEX idx_skills_embedding ON skills USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
```

## TDD plan

1. **TestCrearSkillPrompt:** POST con tipo prompt → 201 + embedding null (porque mock no responde)
2. **TestCrearSkillCode:** POST con tipo code → 201
3. **TestCrearSkillApi:** POST con tipo api, content con url+method → 201
4. **TestCrearSkillMcpTool:** POST con tipo mcp_tool → 201
5. **TestSlugDuplicadoMismoProyecto:** 409
6. **TestSlugDuplicadoDistintoProyecto:** 201 (OK)
7. **TestParametrosSchemaInvalido:** 422
8. **TestParametrosSchemaValido:** 201
9. **TestListarSkillsPorTipo:** filtro type funciona
10. **TestListarSkillsPorTags:** filtro tags overlap funciona
11. **TestObtenerSkillPorID:** 200 con todos los campos
12. **TestObtenerSkillInexistente:** 404
13. **TestActualizarSkill:** PATCH + version incrementa + embedding se regenera
14. **TestEliminarSkillSinDependencias:** 204
15. **TestEliminarSkillConDependencias:** 409
16. **TestSabotajeEmbeddingProviderPanic:** No hay panic, skill se crea con embedding NULL

## Riesgos y mitigación

- **Embedding provider caído:** Skill se crea igual, embedding NULL. Worker reintenta después.
- **JSON Schema muy complejo:** Limitar $ref y profundidad máxima a 10 niveles.
- **Tags array demasiado grande:** Limitar a 20 tags por skill.
- **Slug conflict en alta concurrencia:** Unique constraint a nivel DB lo maneja (error 409).
