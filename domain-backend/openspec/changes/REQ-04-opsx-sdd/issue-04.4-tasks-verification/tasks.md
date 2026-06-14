# Tasks: issue-04.4-tasks-verification

## Backend

- [x] `migrations/XXXX_create_tasks.sql`: tabla + FK a issues + índices
- [x] `migrations/XXXX_create_verification_results.sql`: tabla + FK a tasks
- [x] `migrations/XXXX_create_sabotage_records.sql`: tabla + FK a tasks
- [x] `internal/opsx/task.go`: structs `Task`, `VerificationResult`, `SabotageRecord`, `ProgressReport`
- [x] `internal/store/pg/task.go`: interfaz `TaskStore`
- [x] Implementar `CreateTasks(huID uuid.UUID, tasks []Task) error` (batch con position auto)
- [x] Implementar `ListTasks(huID uuid.UUID) ([]Task, error)` con secciones ordenadas
- [x] Implementar `GetTask(id uuid.UUID) (*Task, error)` con joins a verification + sabotage
- [x] Implementar `UpdateTaskStatus(id uuid.UUID, status string) error` con validación de transición
- [x] Implementar `GetProgress(huID uuid.UUID) (*ProgressReport, error)`
- [x] Implementar `CreateVerification(verification VerificationResult) (uuid.UUID, error)`
- [x] Implementar `CreateSabotage(sabotage SabotageRecord) (uuid.UUID, error)`
- [x] Implementar `ListSabotages(taskID uuid.UUID) ([]SabotageRecord, error)`
- [x] `internal/opsx/task_service.go`: lógica de negocio, validaciones de transición

## Tests

- [x] Test unitario: status transitions (pending→in_progress ok, pending→completed error)
- [x] Test unitario: progress calculation
- [x] Test de integración: crear tareas batch → listar ordenadas
- [x] Test de integración: ciclo completo crear → progreso → completar → verificar
- [x] Test de integración: registrar sabotaje y verificar restored flag
- [x] Test de error: verificar tarea en status pending
- [x] Sabotaje: eliminar FK de tasks → create verification falla

## Cierre

- [x] Verificación manual: crear HU>tareas>completar>verificar>sabotear
- [x] Suite verde
