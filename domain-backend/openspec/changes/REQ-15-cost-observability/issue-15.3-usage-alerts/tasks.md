# Tasks: issue-15.3-usage-alerts

## Backend

- [x] Crear migración para tabla `alerts`
- [x] Crear migración para tabla `alert_log`
- [x] Implementar modelo Alert con validación
- [x] Implementar CRUD de alerts (POST/GET/PUT/DELETE)
- [x] Implementar AlertEvaluator con getMetricValue
- [x] Implementar métrica: cost_per_run (post-ejecución)
- [x] Implementar métrica: cost_per_day (periódica)
- [x] Implementar métrica: tokens_per_minute (periódica)
- [x] Implementar Notifier interface
- [x] Implementar EmailNotifier con SMTP
- [x] Implementar WebhookNotifier con HTTP POST
- [x] Implementar debounce con cooldown configurable
- [x] Integrar evaluador post-ejecución en agent/flow runner
- [x] Implementar ticker periódico (cada 60s) para evaluar métricas acumulativas
- [x] Implementar alert_log con historial de disparos
- [x] Validar configuración de alerta (email formato, threshold > 0)

## Frontend

- [x] N/A (API, UI se cubre en issue-16.1)

## Tests

- [x] Test unitario: evaluador dispara cuando threshold excedido
- [x] Test unitario: evaluador NO dispara cuando threshold no excedido
- [x] Test unitario: debounce impide disparos repetidos
- [x] Test unitario: email notifier envía correctamente
- [x] Test unitario: webhook notifier envía POST con payload correcto
- [x] Test de integración: CRUD alerts
- [x] Test de integración: alert_log se escribe al disparar
- [x] Test de integración: desactivar alerta detiene evaluación
- [x] Sabotaje: evaluador sin cooldown → 10 disparos en 1s → test detecta

## Cierre

- [x] Verificación manual: crear alerta, ejecutar agente costoso, verificar notificación
- [x] Suite verde: `go test ./internal/cost/...`
- [x] Config SMTP documentada en .env.example
