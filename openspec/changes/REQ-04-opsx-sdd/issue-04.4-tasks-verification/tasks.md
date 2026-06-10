# Tasks: issue-04.4-tasks-verification

## Backend

- [ ] `migrations/XXXX_create_tasks.sql`: tabla + FK a issues + índices
- [ ] `migrations/XXXX_create_verification_results.sql`: tabla + FK a tasks
- [ ] `migrations/XXXX_create_sabotage_records.sql`: tabla + FK a tasks
- [ ] `internal/opsx/task.go`: structs `Task`, `VerificationResult`, `SabotageRecord`, `ProgressReport`
- [ ] `internal/store/pg/task.go`: interfaz `TaskStore`
- [ ] Implementar `CreateTasks(huID uuid.UUID, tasks []Task) error` (batch con position auto)
- [ ] Implementar `ListTasks(huID uuid.UUID) ([]Task, error)` con secciones ordenadas
- [ ] Implementar `GetTask(id uuid.UUID) (*Task, error)` con joins a verification + sabotage
- [ ] Implementar `UpdateTaskStatus(id uuid.UUID, status string) error` con validación de transición
- [ ] Implementar `GetProgress(huID uuid.UUID) (*ProgressReport, error)`
- [ ] Implementar `CreateVerification(verification VerificationResult) (uuid.UUID, error)`
- [ ] Implementar `CreateSabotage(sabotage SabotageRecord) (uuid.UUID, error)`
- [ ] Implementar `ListSabotages(taskID uuid.UUID) ([]SabotageRecord, error)`
- [ ] `internal/opsx/task_service.go`: lógica de negocio, validaciones de transición

## Tests

- [ ] Test unitario: status transitions (pending→in_progress ok, pending→completed error)
- [ ] Test unitario: progress calculation
- [ ] Test de integración: crear tareas batch → listar ordenadas
- [ ] Test de integración: ciclo completo crear → progreso → completar → verificar
- [ ] Test de integración: registrar sabotaje y verificar restored flag
- [ ] Test de error: verificar tarea en status pending
- [ ] Sabotaje: eliminar FK de tasks → create verification falla

## Cierre

- [ ] Verificación manual: crear HU>tareas>completar>verificar>sabotear
- [ ] Suite verde
