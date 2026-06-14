# Design: issue-04.1-requirements-crud

## Decisión arquitectónica

**Self-referencing table (parent_id) con CTE recursiva para árbol. Soft-delete via status.**

```
requirements
├── id          UUID PRIMARY KEY DEFAULT gen_random_uuid()
├── slug        VARCHAR(50) UNIQUE NOT NULL      -- "REQ-01-core-platform"
├── title       VARCHAR(500) NOT NULL
├── description TEXT
├── status      VARCHAR(20) NOT NULL DEFAULT 'active'  -- active | archived
├── priority    VARCHAR(20) NOT NULL DEFAULT 'medium'   -- low | medium | high | critical
├── parent_id   UUID REFERENCES requirements(id) ON DELETE SET NULL
├── created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
└── updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
```

**Árbol con CTE recursiva:**
```sql
WITH RECURSIVE req_tree AS (
    SELECT id, slug, title, parent_id, 0 AS depth
    FROM requirements
    WHERE slug = $1
    UNION ALL
    SELECT r.id, r.slug, r.title, r.parent_id, rt.depth + 1
    FROM requirements r
    INNER JOIN req_tree rt ON r.parent_id = rt.id
    WHERE rt.depth < 10  -- max depth safety
)
SELECT * FROM req_tree ORDER BY depth, slug;
```

**Archive cascade:**
```sql
UPDATE requirements SET status = 'archived', updated_at = now()
WHERE id = $1 OR parent_id = $1;
```

**Índices:**
- `requirements_slug_idx` UNIQUE BTREE (slug)
- `requirements_status_idx` BTREE (status)
- `requirements_priority_idx` BTREE (priority)
- `requirements_parent_id_idx` BTREE (parent_id)

## Alternativas descartadas

| Alternativa | Motivo de descarte |
|-------------|-------------------|
| Closure table (path enumeration) | Más complejo; self-FK + CTE alcanza para la profundidad esperada |
| Nested sets | Difícil de mantener con inserts/updates frecuentes |
| DELETE físico en archive | Soft delete preserva historia y trazabilidad |
| NoSQL (Mongo) | Postgres con CTE resuelve el árbol sin cambiar de tecnología |

## Diagrama

```
RequirementService
  │
  ├── Create(slug, title, desc, priority, parentID)
  │     └── validate slug format, parent exists → store.Create
  │
  ├── GetTree(slug)
  │     └── CTE recursiva → struct arbolado en Go
  │
  ├── List(filter)
  │     └── WHERE dinámico según filtros
  │
  └── Archive(slug, recursive)
        └── UPDATE status = 'archived' [+ cascade hijos]
```

## TDD plan

1. **Red**: Test: crear requisito raíz → get by slug → ok
2. **Green**: Implementar Create + GetBySlug
3. **Red**: Test: crear hijo → parent_id correcto
4. **Green**: Implementar Create con parent validation
5. **Red**: Test: árbol con 3 niveles
6. **Green**: Implementar GetTree con CTE
7. **Red**: Test: archivar con cascade
8. **Green**: Implementar Archive
9. **Sabotaje**: slug duplicado → error UniqueViolation

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| CTE recursiva sin límite | Depth limit hardcoded (10) |
| Archivar padre con hijos activos | Por defecto no cascade; opcional con flag explícito |
| Slug mal formateado | Validación regex `^REQ-\d+(-[a-z0-9-]+)?$` |
