# Proposal: issue-15.3-usage-alerts

## Intención

Implementar un sistema de alertas configurables que monitorea métricas de uso (costo por corrida, costo por día, tokens por minuto) y notifica via email o webhook cuando se superan umbrales.

## Scope

**Incluye:**
- CRUD de alertas: POST/GET/PUT/DELETE /api/v1/cost/alerts
- Evaluación de alertas: post-ejecución (inmediata) y periódica (cada minuto)
- Métricas: cost_per_run, cost_per_day, tokens_per_minute
- Canales: email (SMTP), webhook (HTTP POST)
- Debounce: cooldown de 1 hora por alerta
- Alert log: historial de disparos
- Validación de config: email formato, webhook URL, threshold > 0

**Excluye:**
- SMS / Slack / PagerDuty (extensible via webhook)
- Alertas basadas en ML (anomaly detection)
- Dashboard de alertas (issue-16.1)

## Enfoque técnico

**Alerta model:**
```go
type Alert struct {
    ID          string   `json:"id"`
    Name        string   `json:"name"`
    Metric      string   `json:"metric"`       // cost_per_run, cost_per_day, tokens_per_minute
    Condition   string   `json:"condition"`    // greater_than, less_than
    Threshold   float64  `json:"threshold"`
    Channel     string   `json:"channel"`      // email, webhook
    Recipients  []string `json:"recipients"`   // emails or webhook URLs
    Cooldown    int      `json:"cooldown"`     // minutes, default 60
    IsActive    bool     `json:"is_active"`
    LastFiredAt *time.Time `json:"last_fired_at,omitempty"`
    FireCount   int      `json:"fire_count"`
    CreatedAt   time.Time `json:"created_at"`
}
```

**Alert Evaluator:**
```go
type AlertEvaluator struct {
    store         AlertStore
    tokenUsage    TokenUsageStore
    notifier      Notifier
}

func (e *AlertEvaluator) Evaluate(ctx context.Context) {
    alerts, _ := e.store.ListActive(ctx)
    for _, alert := range alerts {
        value, err := e.getMetricValue(ctx, alert.Metric)
        if err != nil { continue }
        if e.shouldFire(alert, value) {
            e.fire(ctx, alert, value)
        }
    }
}
```

**Notifier interface:**
```go
type Notifier interface {
    Send(ctx context.Context, alert Alert, value float64) error
}

type EmailNotifier struct {
    smtpHost string
    smtpPort int
    from     string
    password string
}

type WebhookNotifier struct {
    client *http.Client
}
```

**Trigger points:**
1. Post-ejecución: después de cada domain_agent_run/domain_flow_run, evaluar cost_per_run
2. Periódico: goroutine cada 60s evalua cost_per_day y tokens_per_minute

## Riesgos

| Riesgo | Mitigación |
|--------|------------|
| Email service no configurado | Alert log visible en API aunque email falle |
| Webhook endpoint lento/no responde | Timeout 10s, no bloquear evaluador |
| Demasiadas alertas en poco tiempo | Debounce con cooldown configurable |
| SMTP credentials en config | Encriptar en secrets store (REQ-02.3) |

## Testing

- Unit: alert evaluation logic (debe disparar / no disparar)
- Unit: debounce timing
- Integration: CRUD alerts
- Integration: notifier mock (email + webhook)
- Sabotaje: alert sin cooldown → se dispara múltiples veces
