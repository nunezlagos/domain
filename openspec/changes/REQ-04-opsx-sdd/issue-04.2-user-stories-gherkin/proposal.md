# Proposal: issue-04.2-user-stories-gherkin

## Intención

Implementar gestión de issues (HUs) con escenarios Gherkin almacenados como structured data, vinculados a requisitos (REQs). Los escenarios se almacenan con campos separados (feature, scenario, given[], when, then[]) para permitir consultas y trazabilidad granular.

## Scope

**Incluye:**
- Tabla `issues` con: `id` (UUID), `slug` (TEXT UNIQUE, formato "issue-NN.N-slug"), `title`, `description`, `status` (proposed|active|completed|archived), `priority`, `req_id` (FK a requirements), `created_at`, `updated_at`
- Tabla `gherkin_scenarios` con: `id` (UUID), `issue_id` (FK), `feature` (VARCHAR), `scenario` (VARCHAR), `given` (TEXT[]), `when` (TEXT), `then` (TEXT[]), `position` (INT), `created_at`
- CRUD completo para HUs y escenarios
- Filtros: req_slug, status, priority
- Slug único global (formato validado)

**Excluye:**
- Ejecución de escenarios (solo almacenamiento)
- Gherkin parser from text (se crea mediante API estructurada)
- Template de HU predefinidos

## Enfoque técnico

1. **Migraciones**: 2 tablas con FKs
2. **Gherkin como structured data**: array de strings para Given/Then (múltiples pasos)
3. **Position**: orden de escenarios dentro de la HU
4. **Capa Go**: `UserStoryStore` + `GherkinStore` + `UserStoryService`

## Riesgos

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| Scenario con Given/Then vacío | Bajo | Validar al menos 1 Given y 1 Then |
| HU huérfana si se elimina REQ | Medio | FK con ON DELETE RESTRICT |
| Orden de escenarios inconsistente | Bajo | Position secuencial auto-asignada |

## Testing

- **Unitarios**: validación de estructura Gherkin
- **Integración**: crear HU con escenarios, listar, filtrar
- **Sabotaje**: eliminar REQ con HUs → error FK
