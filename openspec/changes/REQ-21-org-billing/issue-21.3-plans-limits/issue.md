# issue-21.3-plans-limits

**Origen:** `REQ-21-org-billing`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** plataforma
**Quiero** definir planes con límites por dimensión y aplicarlos
**Para** monetizar y prevenir abuso

## Criterios de aceptación

### Escenario 1: Planes definidos

```gherkin
Dado que existen 3 planes seed: Free, Pro, Enterprise
Cuando inspecciono tabla `plans`
Entonces los límites por defecto son:
  | plan       | tokens/mes | runs/mes | storage_gb | members | seats |
  | Free       | 100_000    | 100      | 1          | 3       | 1     |
  | Pro        | 5_000_000  | 5_000    | 50         | 25      | 10    |
  | Enterprise | ilimitado  | ilimitado| 500        | ilim    | ilim  |
```

### Escenario 2: Tracking de consumo

```gherkin
Dado que org está en plan Free
Cuando un agente consume 1000 tokens
Entonces se incrementa `usage.tokens_this_month` en 1000
Y se compara contra `plan.tokens_per_month`
```

### Escenario 3: Throttle al exceder soft limit (80%)

```gherkin
Dado que org está en plan Free y consumió 80_000 / 100_000 tokens (80%)
Cuando se intenta nuevo run
Entonces se ejecuta normalmente
Y se notifica al canal admin (REQ-20) "Usage at 80%"
Y métrica `domain_usage_warning_total{dimension="tokens"}` se incrementa
```

### Escenario 4: Block al exceder hard limit (100%)

```gherkin
Dado que org consumió 100_000 / 100_000 tokens
Cuando se intenta nuevo run que consumiría tokens
Entonces el run falla con error 402 "quota exceeded: tokens"
Y se notifica admin "Hard limit exceeded"
Y la org no puede ejecutar más runs hasta upgrade o reset mensual
```

### Escenario 5: Reset mensual

```gherkin
Dado que ahora es el 1ro del mes
Cuando se ejecuta el cron de reset
Entonces `usage.tokens_this_month`, `runs_this_month` vuelven a 0
Y `usage.storage_gb` y `members` NO se resetean (son acumulativas)
```

### Escenario 6: Custom limits per-org

```gherkin
Dado que org Enterprise negoció 10_000_000 tokens/mes
Cuando admin platform setea `organizations.custom_limits` JSONB
Entonces se respetan custom_limits sobre plan defaults
```

## Análisis breve

- **Qué pide:** plans table + usage tracking + throttle/block + reset cron + custom limits
- **Esfuerzo:** M
- **Riesgos:** race en counter (use atomic counters); cron mal timezone
