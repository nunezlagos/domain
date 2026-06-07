# Proposal: HU-09.9-saga-compensation

## Intención

Implementar saga pattern: cada step puede declarar compensación; si el flow aborta tras N steps completos, motor ejecuta compensaciones en reverso. Idempotency + retry + failure tracking + manual skip.

## Scope

**Incluye:**
- Field `compensate` por step (skill_slug o inline)
- Executor reverso con retry policy
- Tabla `flow_compensation_failures` para intervención manual
- Endpoint admin skip
- Flag `compensate_in_parallel` opcional
- Audit completo

**No incluye:**
- Distributed sagas cross-services (single platform)
- Auto-rollback de DB (responsabilidad del step)

## Enfoque técnico

1. Engine detecta status=failed → trigger compensation phase
2. Itera completed steps en reverse order
3. Cada compensate respeta retry policy
4. Failures persistidas, NO bloquean otras
5. Status final: `failed | failed_compensated | failed_compensation_failed`

## Riesgos

- Idempotencia compensación: responsabilidad del developer; documentar fuerte
- Loops: NO permitir compensaciones disparen sub-flows
- Performance: cap compensación duration

## Testing

- Happy: A,B,C complete, D fails → compensate C,B,A
- Compensación fails → tabla failures + notif
- Skip manual
- Parallel mode
- Idempotency: re-trigger compensation no duplica side effects
