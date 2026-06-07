# Proposal: HU-01.4-project-templates

## Intención

Permitir que cada proyecto tenga su propia "forma de trabajo" mediante templates reutilizables. Un template define skills default, scope de memoria, agentes preconfigurados y otros settings que se aplican al crear un proyecto. Esto resuelve el problema de que "abc1" y "abc-docker" usen configuraciones diferentes sin tener que configurar manualmente cada proyecto.

## Scope

**Incluye:**
- Tabla `project_templates` con: id, name, description, is_default, settings (JSONB), default_skills (TEXT[]), created_at
- CRUD de templates: create, list, get, update, delete
- Seed de templates por defecto: "default" (genérico), "go-backend", "frontend", "data-pipeline"
- Al crear proyecto, opción de template_id
- Copia de settings del template al proyecto (no link)
- CLI: `domain project-template create/list/get/update/delete`
- Validación: name único, settings validados con schema

**Excluye:**
- Templates compartidos entre organizaciones
- Versionado de templates
- Marketplace de templates de comunidad

## Enfoque técnico

1. Migración 000021 para crear `project_templates`
2. Seed data con templates built-in
3. Service layer con validación de settings JSONB
4. Al crear proyecto, merge de settings: template settings como defaults, override explícito gana
5. CLI con cobra

## Riesgos

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| Template cambia y proyectos existentes quedan obsoletos | Bajo | Copy-on-create (no link). Si se necesita upgrade, será feature futuro |
| JSONB settings sin schema de validación | Medio | Schema registry con validación al guardar template |
| Demasiados templates fragmentan la experiencia | Bajo | Template "default" siempre presente, máx 50 por org |

## Testing

- **Unitarios**: CRUD de templates con store mockeado
- **Integración**: Crear proyecto desde template, verificar skills asignados
- **Regression**: Template con settings inválidos → error de validación
- **Sabotaje**: Eliminar template usado por proyectos existentes → debe permitirse (copy-on-create)
