# Proposal: issue-27.3-hot-reload-config

## Intención

Tabla `runtime_configs` editable via admin API que propaga cambios cross-pod sin restart vía NOTIFY (issue-26.7 pattern).

## Scope

- Migration runtime_configs
- Admin endpoints CRUD
- Validators per-config registrados
- NOTIFY-based propagation (reuse issue-26.7)
- Apply hooks por config (log level swap, timeouts update, etc.)
- Audit log
- Seed inicial via issue-01.7

## Riesgos

- Apply hook bug: tests + log warn si apply falla
- Non-reloadable cambio sin warn: API 409 explícito

## Testing

- Cambio aplicado en todos los pods
- SIGHUP reload fallback
- Validation rechaza inválidos
- Non-reloadable 409
- Audit log
