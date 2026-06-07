# Proposal: HU-04.1-requirements-crud

## Intención

Implementar CRUD completo de requisitos (REQs) con jerarquía padre-hijo, filtros, archive y árbol. Es la base del sistema SDD sobre el que se construyen HUs, specs, diseños y trazabilidad.

## Scope

**Incluye:**
- Tabla `requirements` con: `id` (UUID), `slug` (TEXT UNIQUE, formato "REQ-NN-slug"), `title`, `description`, `status` (active|archived), `priority` (low|medium|high|critical), `parent_id` (UUID, self-FK nullable), `created_at`, `updated_at`
- CRUD: Create, GetBySlug, GetByID, Update, Archive (cascade opcional), List
- List con filtros: status, priority, search (title/description LIKE)
- Árbol jerárquico: GetTree(slug) que devuelve nodo + hijos recursivos
- Validación: slug único, parent_id debe existir y no ser archived
- Índices: slug, status, priority, parent_id

**Excluye:**
- Reordenamiento de prioridades
- Asociación con HUs (es HU-04.2)
- Import/export

## Enfoque técnico

1. **Migración**: `CREATE TABLE requirements (id UUID PK, slug TEXT UNIQUE, title TEXT, description TEXT, status VARCHAR(20), priority VARCHAR(20), parent_id UUID REFERENCES requirements(id), created_at, updated_at)`
2. **Árbol**: query recursiva CTE o múltiples queries con gather en Go
3. **Archive cascade**: opcional, configurable con flag `recursive`
4. **Capa Go**: `RequirementStore` interface, `RequirementService` con lógica de negocio

## Riesgos

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| Árbol muy profundo (>10 niveles) | Bajo | Límite de profundidad configurable |
| Archive cascade elimina datos | Medio | Soft delete (status = archived), no DELETE físico |

## Testing

- **Unitarios**: service con store mockeado
- **Integración**: pgtest, ciclo completo create → update → archive → tree
- **Sabotaje**: archivar padre sin cascade → hijos siguen activos
