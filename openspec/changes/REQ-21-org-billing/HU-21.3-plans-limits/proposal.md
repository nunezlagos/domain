# Proposal: HU-21.3-plans-limits

## Intención

Modelar planes (Free/Pro/Enterprise) con límites por dimensión, trackear consumo, throttle al 80% (notificación) y block al 100% (rechazar requests), reset mensual y custom limits por org.

## Scope

**Incluye:**
- Tabla `plans` con limits JSONB por dimensión
- Tabla `usage_counters` particionada por mes
- Servicio `quota.Check(orgID, dimension, amount)` antes de operaciones costosas
- Cron reset mensual con timezone configurable
- Custom limits per-org en `organizations.custom_limits` JSONB
- Notificaciones soft/hard via REQ-20

**No incluye:**
- Integración Stripe billing (HU-21.4)
- Multi-currency
- Trials/promo codes (futuro)

## Enfoque técnico

1. `usage_counters` con upsert atómico (`INSERT ... ON CONFLICT DO UPDATE SET amount = amount + EXCLUDED.amount`)
2. Check antes (estimate) y record después (real) para tokens
3. Particionado mensual para borrar histórico viejo eficiente
4. Cron reset usa timezone de la org (default UTC)

## Riesgos

- Race: counter atómico
- Subestimación tokens pre-run → check después también
- Reset cron no corre → fallback en cada check

## Testing

- Consumo lineal hasta 80% → notif soft, no block
- Consumo > 100% → block 402
- Cron reset → counters a 0
- Custom limits override plan
