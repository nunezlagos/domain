# Proposal: HU-09.8-sync-guidance

## Intención

Crear un sistema de sync guidance que clasifique errores de cloud sync como reparables o no, y genere mensajes de ayuda con pasos concretos y comandos que el usuario puede ejecutar (doctor, repair, etc.).

## Scope

**Incluye:**
- `IsRepairableCloudSyncError(err) bool` — clasificación por error code
- `BuildGuidance(err) string` — mensaje formateado con título, descripción, pasos, comandos
- Cobertura de códigos: auth_expired, network_timeout, sync_conflict, rate_limited
- Non-repairable: internal_error, unknown, cualquier otro no mapeado
- Estructura de mensaje estandarizada (i18n-ready)

**No incluye:**
- Ejecución automática de doctor/repair (solo guidance)
- Traducciones (solo inglés por ahora)
- Logging o métricas de errores

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Clasificación | Switch por error code; códigos conocidos → reparable |
| Guidance | Template string builders con título, descripción, steps, commands |
| Comandos | `engram doctor`, `engram repair`, `engram cloud auth` |
| Error codes | String enum en package de errores de sync |
