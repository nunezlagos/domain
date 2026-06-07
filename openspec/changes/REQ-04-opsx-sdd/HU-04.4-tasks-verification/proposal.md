# Proposal: HU-04.4-tasks-verification

## Intención

Implementar gestión de tareas por HU con tracking de estado, verificación, sabotaje y cálculo de progreso. Cada HU tiene múltiples tareas organizadas por secciones (Backend, Tests, Cierre), con resultados de verificación y registros de sabotaje.

## Scope

**Incluye:**
- Tabla `tasks` con: `id`, `hu_id` (FK), `section` (VARCHAR), `description` (TEXT), `status` (pending|in_progress|completed), `position` (INT), `started_at`, `completed_at`, `completed_by`, `created_at`, `updated_at`
- Tabla `verification_results` con: `id`, `task_id` (FK), `result` (pass|fail|blocked), `evidence` (TEXT), `notes` (TEXT), `verified_at`, `verified_by`
- Tabla `sabotage_records` con: `id`, `task_id` (FK), `action` (TEXT), `expected_failure` (TEXT), `actual_result` (TEXT), `restored` (BOOLEAN), `performed_at`
- CRUD: crear tareas batch, actualizar status, registrar verificación, registrar sabotaje
- Progress: COUNT tareas total vs COUNT completed
- Query: tareas con verificación y sabotaje join

**Excluye:**
- Auto-ejecución de sabotajes
- Pipeline CI/CD
- Asignación a usuarios

## Enfoque técnico

1. **Migraciones**: 3 tablas secuenciales
2. **Batch create**: INSERT multiple con position auto-asignado por section
3. **Status transitions**: validar pending → in_progress → completed (no saltos)
4. **Progress query**: `SELECT hu_id, COUNT(*) as total, COUNT(*) FILTER (WHERE status = 'completed') as completed FROM tasks GROUP BY hu_id`
5. **Capa Go**: `TaskStore` interface con métodos para task, verification, sabotage

## Riesgos

| Riesgo | Impacto | Mitigación |
|--------|---------|------------|
| Status transition inválida | Medio | Validación en service layer |
| Sabotaje sin restauración | Medio | Flag restored en registro; notificar si false |
| Verification sin tarea completada | Bajo | Validar que task status = completed antes de verificar |

## Testing

- **Unitarios**: status transitions, progress calc
- **Integración**: crear tareas → completar → verificar → sabotear
- **Sabotaje**: verificar tarea no completada → error
