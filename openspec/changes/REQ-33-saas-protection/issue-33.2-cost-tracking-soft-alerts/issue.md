# issue-33.2-cost-tracking-soft-alerts

**Origen:** `REQ-33-saas-protection`
**Prioridad tentativa:** media
**Tipo:** feature (operational)

## Historia de usuario

**Como** operador del VPS multi-tenant
**Quiero** recibir un email de alerta cuando una org acumula más de X USD en costos LLM en un día
**Para** enterarme ANTES de que el cliente nos gaste todo el presupuesto del mes — y poder contactarlo o tomar acción manual

## Criterios de aceptación

### Escenario 1: Alerta al cruzar threshold

```gherkin
Dado que org A tiene config `cost_alert_daily_usd = 50`
Y la org A gastó $49.50 a las 14:00
Y la org A hace una request que suma $1.00 (total $50.50)
Entonces el sistema:
  1. Calcula el total del día
  2. Compara con threshold
  3. Si crossed → email al admin (configurado en `DOMAIN_ADMIN_EMAIL`)
  4. NO bloquea la request (es soft, no hard cap)
Y el email tiene subject: "[domain] cost alert: org A exceeded $50/day (now $50.50)"
```

### Escenario 2: Threshold configurable per-org

```gherkin
Dado que org A tiene `cost_alert_daily_usd = 10` (cliente chico, sensible)
Y org B tiene `cost_alert_daily_usd = 500` (cliente grande, threshold alto)
Cuando cada una cruza su threshold respectivo
Entonces cada una recibe su alerta INDEPENDIENTEMENTE
Y el admin puede ver en el log a quién se le envió
```

### Escenario 3: Default + dedup anti-spam

```gherkin
Dado que una org sin config explícita
Cuando cruza el default $100/día
Entonces recibe la alerta
Y si sigue gastando (sin pausa), NO recibe otra alerta hasta el día siguiente
Y el sistema tiene un "alerted_at_<date>" marker para evitar spam
```

### Escenario 4: Job cron horario

```gherkin
Dado que el job `runUsageAlertEvaluator` corre cada 5 min (ya existe, ver design REQ-15.3)
Y procesa agregaciones de cost_logs
Cuando detecta una org que crossed threshold y no se le envió alerta hoy
Entonces emite el email y registra `cost_alerts` con timestamp
```

### Escenario 5: Tracking a nivel de provider/model

```gherkin
Dado que una org gasta $50 en Anthropic y $30 en OpenAI en un día
Y el threshold es $100
Cuando llega la alerta
Entonces el email incluye breakdown:
  - Anthropic: $50 (Claude Sonnet: 1M tokens, $30; Claude Haiku: 5M tokens, $20)
  - OpenAI: $30 (GPT-4: 0.5M tokens, $30)
Y el admin ve de dónde viene el costo
```

### Escenario 6: Sin SMTP configurado → fallback log

```gherkin
Dado que `DOMAIN_SMTP_HOST` NO está seteada
Y una org cruza threshold
Cuando el job intenta enviar el email
Entonces loggea: "cost alert for org A: $50.50/day, threshold $50, but SMTP not configured; alert logged only"
Y exit 0 (no crashea el job)
```

### Escenario 7: Sabotaje — alerta bloquea la request

```gherkin
Dado que el código de alerta tiene un bug (sabotaje) que en vez de notificar BLOQUEA la request
Y una org cruza threshold
Cuando hace una nueva request
Entonces recibe 429 con "cost limit exceeded" (incorrecto, es soft)
Y el test e2e que assserta "alerta NO bloquea" DEBE FALLAR
Cuando restauro la lógica de solo notificar
Entonces el test verde
```

### Escenario 8: Edge case — UTC vs local timezone

```gherkin
Dado que la org está en zona horaria America/Argentina/Buenos_Aires (UTC-3)
Y el threshold se evalúa en UTC
Cuando a las 22:00 ART (= 01:00 UTC del día siguiente) cruza threshold
Entonces la alerta dice "fecha UTC" (e.g. "2026-06-12 UTC: $50")
Y NO se confunde con la "fecha local" del user
Y el reset del "alerted_at_<date>" usa UTC midnight
```

## Notas

- Tracking de costos YA EXISTE en `cost_logs` (issue-15.3). El
  job `runUsageAlertEvaluator` corre cada 5 min. La feature
  nueva es:
  1. Threshold per-org (default 100 USD/día).
  2. Email notification (no bloqueante).
  3. Anti-spam (1 alerta por org por día).
- El job YA EXISTE. Solo se le agrega la lógica de threshold +
  email. No se duplica el job.
- NO hay "hard cap" (bloquear requests cuando excede). Es solo
  soft alert. Decisión del usuario explícita: "no paywall".
