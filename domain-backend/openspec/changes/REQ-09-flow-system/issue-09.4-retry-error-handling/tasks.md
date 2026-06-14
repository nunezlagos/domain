# Tasks: issue-09.4-retry-error-handling

## Backend

- [x] Implementar backoff exponencial + fixed → runner/flow/retry.go buildRetryPlan (closures delay; sin interface — YAGNI) — 2026-06-10
- [x] Implementar modelo retry → flow.StepRetryPolicy en Step.Retry (vive con su feature, no en models/) — 2026-06-10
- [x] Implementar loop de reintento → Runner.runStepWithRetry (acumula attempt errors + retry_count) — 2026-06-10
- [x] Implementar las 4 políticas de error → switch en Run loop: ignore_and_continue/continue, abort_flow/fail, fallback_step, jump a step_id — 2026-06-10
- [x] Integrar retry en flow runner → runStepWithRetry reemplaza el loop legacy (que se preserva como plan legacy con gate transient) — 2026-06-10
- [x] Integrar error handler en flow runner → políticas post-agotamiento con prioridad step > flow default — 2026-06-10
- [x] Agregar campo `default_step_error_policy` → Spec.DefaultStepErrorPolicy (JSONB, sin migración) — 2026-06-10
- [x] Crear migración SQL para tabla `dead_letter_queue` → 000079 — 2026-06-10
- [x] Implementar `DLQRepository` → flow.DLQStore (Insert, List pendientes, Resolve) — 2026-06-10
- [x] Crear handler REST: GET /api/v1/dlq → listDLQ (limit param) — 2026-06-10
- [x] Crear handler REST: DELETE /api/v1/dlq/:id → resolveDLQ (204; 404 si no existe) — 2026-06-10
- [x] Agregar validación: fallback_step requiere fallback_step definido → validateErrorHandling — 2026-06-10
- [x] Agregar validación: ignore_and_continue requiere default_on_error → validateErrorHandling — 2026-06-10
- [x] Agregar límite de 3 niveles de profundidad de fallback → MaxFallbackDepth (validación + runtime) — 2026-06-10
- [x] Agregar límite máximo de max_retries (10) → MaxRetriesCap (validación + cap en plan) — 2026-06-10

## Tests

- [x] Test unitario: ExponentialBackoff delays correctos (1, 2, 4s) → TestBuildRetryPlan_ExponentialBackoff
- [x] Test unitario: FixedBackoff delays constantes → TestBuildRetryPlan_FixedBackoff
- [x] Test unitario: ShouldRetry con retry_on match (timeout) → TestBuildRetryPlan_RetryOnFilter
- [x] Test unitario: ShouldRetry con retry_on no match (validation_error) → TestBuildRetryPlan_RetryOnFilter
- [x] Test unitario: ShouldRetry con retry_on vacío (todos reintentan) → TestBuildRetryPlan_EmptyRetryOn_RetriesAll
- [x] Test unitario: max_retries alcanzado → TestErrorPolicy_PermanentFailure_GoesToDLQ (retry_count=2, 3 errores)
- [x] Test unitario: clasificación de errores → TestClassifyError_Matrix
- [x] Test unitario: ignore_and_continue con default_on_error → TestErrorPolicy_IgnoreAndContinue_UsesDefault
- [x] Test unitario: abort_flow detiene ejecución → TestErrorPolicy_PermanentFailure_GoesToDLQ (status failed)
- [x] Test unitario: fallback_step se ejecuta y reemplaza resultado → TestErrorPolicy_FallbackStep_Succeeds (fallback_used=true)
- [x] Test unitario: fallback_step recursivo máximo 3 niveles → TestValidate_FallbackChainDepthLimit (3 OK, 4 rechazado)
- [x] Test unitario: política de step sobreescribe política del flow → TestErrorPolicy_FlowDefaultApplied + validación escenario 8
- [x] Test unitario: DLQ Create registra error permanente → TestErrorPolicy_PermanentFailure_GoesToDLQ
- [x] Test unitario: DLQ List + Resolve → mismo test (lista, resuelve, 404 en re-resolve)
- [x] Test de integración: step con retry → eventual failure → DLQ → TestErrorPolicy_PermanentFailure_GoesToDLQ
- [x] Test de integración: fallback falla → abort + DLQ → TestErrorPolicy_FallbackFails_Aborts
- [x] Sabotaje: max_retries sin límite → TestValidate_MaxRetriesCap (11 rechazado) + TestBuildRetryPlan_CapsMaxRetries

## Cierre

- [x] Verificación manual: retry_on timeout, falla con validation → no retry → TestBuildRetryPlan_RetryOnFilter cubre la matriz
- [x] Verificación manual: DLQ aparece después de error permanente → verificado E2E en integración
- [x] Suite verde → 2026-06-10
