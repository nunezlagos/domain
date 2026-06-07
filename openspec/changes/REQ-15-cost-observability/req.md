# REQ-15-cost-observability: Métricas y costos: tracking de tokens por run/agente/flow, agregación por proyecto, analytics, alertas de costo.

**Estado:** activo
**Creado:** 2026-06-07
**Fase:** F3

## Descripción

Métricas y costos: tracking de tokens por run/agente/flow, agregación por proyecto, analytics, alertas de costo.

## Criterios de éxito

- Token tracking por run, agente, flow con timestamps
- Cost analytics con agregación por proyecto, período y modelo
- Alertas configurables por umbral de costo

## HUs hijas

| HU | Estado | Descripción |
|----|--------|-------------|
| HU-15.1-token-tracking | proposed | Tracking de tokens por operación: prompt, completion, modelo, costo estimado |
| HU-15.2-cost-analytics | proposed | Agregación de costos por proyecto/período/modelo, reportes exportables |
| HU-15.3-usage-alerts | proposed | Alertas configurables por umbral de costo, notificaciones |
