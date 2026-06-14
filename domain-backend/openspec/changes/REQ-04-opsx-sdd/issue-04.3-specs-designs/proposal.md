# Proposal: issue-04.3-specs-designs

## Intención

Implementar almacenamiento de especificaciones (proposals) y diseños técnicos por HU, con contenido en markdown y metadatos estructurados. Versionado simple para tracking de cambios. Status workflow: draft → approved | rejected.

## Scope

**Incluye:**
- Tabla `proposals` con: `id`, `issue_id` (FK), `version` (INT), `status` (draft|approved|rejected), `intention`, `scope`, `approach` (TEXT, markdown), `risks` (TEXT, markdown), `testing_notes`, `rejection_reason` (nullable), `created_at`, `updated_at`
- Tabla `designs` con: `id`, `issue_id` (FK), `proposal_id` (FK nullable), `version` (INT), `status` (draft|final), `arch_decisions` (TEXT, markdown), `alternatives` (TEXT, markdown), `data_flow` (TEXT, markdown), `tdd_plan` (TEXT, markdown), `risks_mitigation` (TEXT, markdown), `created_at`, `updated_at`
- Versionado: al actualizar, INSERT nuevo registro con version+1
- CRUD: crear, obtener última versión, obtener versión específica, listar por HU, cambiar status
- Rollback de versiones no (solo lectura de versiones anteriores)

**Excluye:**
- Diff entre versiones
- Aprobación multi-step
- Templates de proposal/design predefinidos

## Enfoque técnico

1. **Migraciones**: 2 tablas con versionado por INSERT (no UPDATE)
2. **Versionado**: cada UPDATE es un INSERT con version+1; "current" = MAX(version) GROUP BY issue_id
3. **Markdown**: TEXT sin validación de formato; se renderiza en UI (futuro)
4. **Capa Go**: `SpecStore` interfaz unificada o separada `ProposalStore` + `DesignStore`

## Riesgos

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| Crecimiento rápido por versionado | Bajo | Las proposals no cambian frecuentemente; cleanup futuro si es necesario |
| Markdown malicioso | Bajo | Solo agentes internos escriben; sin renderizado directo al usuario aún |
| Proposal sin design | Bajo | Relación opcional; proposal puede existir sin design |

## Testing

- **Unitarios**: versionado, status transitions
- **Integración**: crear proposal → nueva versión → listar versiones
- **Sabotaje**: crear design sin proposal → ok (FK nullable)
