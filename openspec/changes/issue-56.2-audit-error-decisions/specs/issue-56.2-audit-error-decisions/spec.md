# issue-56.2 — Auditoría de decisiones sobre errores

Toda clasificación de un error como benigno (`known_error_set`) y todo reset
(`error_reset`) queda auditado con actor, tiempo y razón; `error_reset` es
soft-delete reversible; y los hooks lifecycle registran sus propios fallos.

## Requisitos

### Requirement: audit trail de known_error_set
`domain_known_error_set` MUST registrar una entrada en `error_decision_log` con
el actor (Principal), el timestamp, la razón y un snapshot de la clasificación.

#### Scenario: clasificar un error como benigno deja rastro
- **Given** un fingerprint válido y una sesión autenticada
- **When** se llama `domain_known_error_set` con `reason`
- **Then** se inserta una fila en `error_decision_log` con `action='known_error_set'`, `actor_id`, `reason` y `detail`

#### Scenario: sesión sin principal
- **Given** una sesión sin Principal autenticado
- **When** se llama `domain_known_error_set`
- **Then** la entrada de audit se registra con `actor_id` NULL (no falla)

### Requirement: error_reset es soft-delete reversible
`domain_error_reset` MUST marcar el `error_event` como borrado
(`deleted_at`/`deleted_by`/`deletion_reason`) en vez de un DELETE físico.

#### Scenario: reset marca soft-delete
- **Given** un `error_event` vivo (`deleted_at IS NULL`)
- **When** se llama `domain_error_reset` con `reason`
- **Then** la fila queda con `deleted_at`, `deleted_by` y `deletion_reason` seteados
- **And** la fila NO se elimina físicamente de la tabla
- **And** se registra una entrada en `error_decision_log` con `action='error_reset'`

#### Scenario: reset repetido no pisa la autoría original
- **Given** un `error_event` ya soft-deleted
- **When** se llama `domain_error_reset` de nuevo sobre el mismo fingerprint
- **Then** el UPDATE afecta 0 filas (solo toca filas con `deleted_at IS NULL`)

### Requirement: los hooks lifecycle registran sus fallos
Los hooks `domain-stop.sh` y `domain-user-prompt.sh` MUST registrar en
`hook-errors.log` cuando su llamada al server falla, en vez de silenciarla.

#### Scenario: fallo de turn_complete queda auditado
- **Given** que `domain_turn_complete` devuelve error o el curl falla
- **When** corre `domain-stop.sh`
- **Then** se agrega una línea a `~/.local/state/domain/hook-errors.log` con hook, sesión, operación y detalle
- **And** el hook igual termina con exit 0 (best-effort, no bloquea)
