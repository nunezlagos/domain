# Tasks: HU-15.3-usage-alerts

## Backend

- [ ] Crear migración para tabla `alerts`
- [ ] Crear migración para tabla `alert_log`
- [ ] Implementar modelo Alert con validación
- [ ] Implementar CRUD de alerts (POST/GET/PUT/DELETE)
- [ ] Implementar AlertEvaluator con getMetricValue
- [ ] Implementar métrica: cost_per_run (post-ejecución)
- [ ] Implementar métrica: cost_per_day (periódica)
- [ ] Implementar métrica: tokens_per_minute (periódica)
- [ ] Implementar Notifier interface
- [ ] Implementar EmailNotifier con SMTP
- [ ] Implementar WebhookNotifier con HTTP POST
- [ ] Implementar debounce con cooldown configurable
- [ ] Integrar evaluador post-ejecución en agent/flow runner
- [ ] Implementar ticker periódico (cada 60s) para evaluar métricas acumulativas
- [ ] Implementar alert_log con historial de disparos
- [ ] Validar configuración de alerta (email formato, threshold > 0)

## Frontend

- [ ] N/A (API, UI se cubre en HU-16.1)

## Tests

- [ ] Test unitario: evaluador dispara cuando threshold excedido
- [ ] Test unitario: evaluador NO dispara cuando threshold no excedido
- [ ] Test unitario: debounce impide disparos repetidos
- [ ] Test unitario: email notifier envía correctamente
- [ ] Test unitario: webhook notifier envía POST con payload correcto
- [ ] Test de integración: CRUD alerts
- [ ] Test de integración: alert_log se escribe al disparar
- [ ] Test de integración: desactivar alerta detiene evaluación
- [ ] Sabotaje: evaluador sin cooldown → 10 disparos en 1s → test detecta

## Cierre

- [ ] Verificación manual: crear alerta, ejecutar agente costoso, verificar notificación
- [ ] Suite verde: `go test ./internal/cost/...`
- [ ] Config SMTP documentada en .env.example
