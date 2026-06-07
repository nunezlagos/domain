# Design: HU-01.4-project-templates

## Decisión arquitectónica

**Copy-on-create: los settings del template se copian al proyecto en el momento de creación. No hay link en vivo.**

Esto significa que:
- Cambiar un template NO afecta proyectos existentes
- Cada proyecto es independiente después de creado
- Si se necesita "actualizar" un proyecto a un template nuevo, será un comando explícito a futuro

```
project_templates
├── id           UUID PK DEFAULT gen_random_uuid()
├── name         VARCHAR(255) NOT NULL UNIQUE
├── description  TEXT
├── is_default   BOOLEAN NOT NULL DEFAULT false
├── settings     JSONB NOT NULL DEFAULT '{}'
│   ├── memory_default_scope     "project" | "personal" | "global"
│   ├── memory_dedup_enabled     boolean
│   ├── memory_dedup_window       int
│   └── ...
├── default_skills TEXT[] NOT NULL DEFAULT '{}'
├── created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
└── updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
```

**Seed templates built-in:**

| Template | default_skills | memory scope |
|----------|---------------|-------------|
| default | [] | project |
| go-backend | [code-review, architecture, docker] | project |
| frontend | [ux-review, architecture, css] | personal |
| data-pipeline | [sql, docker, monitoring] | project |

## Alternativas descartadas

| Alternativa | Motivo |
|-------------|--------|
| Link en vivo (template changes propagate) | Rompería proyectos existentes inesperadamente |
| Sin templates, solo config manual | Mala UX, cada proyecto requiere setup tedioso |
| Templates solo en seed (hardcoded) | Inflexible, el usuario no puede crear los suyos |

## TDD plan

1. **Red**: Test de crear template con settings válidos
2. **Green**: Implementar migración + store.Create
3. **Red**: Test de crear proyecto desde template verifica skills copiados
4. **Green**: Implementar ProjectService.CreateFromTemplate
5. **Red**: Test de template con settings inválidos → error
6. **Green**: Validación de schema JSONB
7. **Sabotaje**: Eliminar template → proyectos existentes siguen funcionando

## Riesgos y mitigación

| Riesgo | Mitigación |
|--------|------------|
| JSONB settings sin types | Schema validation con mapa de tipos conocidos |
| Multi-tenant isolation | Templates scoped a organization_id |
