# HU-15.3-usage-alerts

**Origen:** `REQ-15-cost-observability`
**Prioridad tentativa:** baja
**Tipo:** feature

## Historia de usuario
**Como** administrador de la plataforma
**Quiero** configurar alertas que se disparen cuando el consumo de tokens o costos supera umbrales
**Para** recibir notificaciones proactivas antes de que los costos se salgan de control

## Criterios de aceptación

```gherkin
Feature: Usage Alerts

  Background:
    Given existe un sistema de token_usage con datos históricos

  Scenario: Crear alerta por costo por corrida
    When creo una alerta con:
      | campo        | valor                          |
      | metric       | cost_per_run                   |
      | threshold    | 0.50                           |
      | condition    | greater_than                   |
      | channel      | email                          |
      | recipients   | admin@example.com              |
    Then la alerta se guarda con estado "active"
    And se evaluará después de cada corrida

  Scenario: Alerta se dispara por cost_per_run
    Given existe una alerta: cost_per_run > $0.50
    When se ejecuta una corrida con costo $0.75
    Then la alerta se dispara
    And se envía notificación por email a admin@example.com
    And se registra en alert_log

  Scenario: Alerta por cost_per_day
    Given existe una alerta: cost_per_day > $10.00
    When el gasto del día actual supera $10.00
    Then la alerta se dispara
    And no se repite hasta el día siguiente

  Scenario: Alerta por tokens_per_minute
    Given existe una alerta: tokens_per_minute > 100000
    When en un minuto se usan más de 100K tokens
    Then la alerta se dispara

  Scenario: Alerta con canal webhook
    Given existe una alerta con channel = webhook
    When la alerta se dispara
    Then se envía POST al webhook configurado con payload JSON
    And el payload incluye: metric, threshold, current_value, timestamp

  Scenario: Alerta no se repite (debounce)
    Given existe una alerta que ya se disparó hace 5 minutos
    When la condición sigue siendo true
    Then no se envía otra notificación
    And la alerta tiene cooldown de 1 hora

  Scenario: Desactivar alerta
    When desactivo una alerta
    Then su estado cambia a "inactive"
    And ya no se evalúa

  Scenario: Listar alertas con estado
    When consulto las alertas
    Then veo todas las alertas con su estado, última vez disparada, contador

  Scenario: Canal email inválido
    When creo una alerta con email inválido
    Then recibo error de validación
    And la alerta no se crea

  Scenario: Threshold negativo
    When creo una alerta con threshold negativo
    Then recibo error de validación
```

## Análisis breve

- **Qué pide realmente:** Sistema de reglas: alerta = métrica + threshold + condición + canal. Evaluación periódica (cada minuto) y post-ejecución. Debounce para evitar spam. Canales: email, webhook.
- **Módulos sospechados:** `internal/cost/alerts/`, `internal/notifications/`
- **Riesgos / dependencias:** Depende de HU-15.1 (datos) y HU-15.2 (agregaciones). Servicio de email requiere configuración SMTP.
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
