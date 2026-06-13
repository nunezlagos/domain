# issue-33.4-quota-snapshot-dashboard-ready

**Origen:** `REQ-33-saas-protection`
**Prioridad tentativa:** media
**Tipo:** feature (read-only API)

## Historia de usuario

**Como** usuario del dashboard web (o developer con curl)
**Quiero** poder consultar mi uso actual de domain (observaciones, agents, flow_runs, costo LLM del día)
**Para** ver cuánto consumí sin tener que preguntar al admin o recibir email

## Criterios de aceptación

### Escenario 1: `GET /api/v1/usage/current` retorna snapshot

```gherkin
Dado que estoy autenticado (API key o sesión)
Cuando hago `GET /api/v1/usage/current`
Entonces el response es 200 con JSON:
  {
    "organization": {id, name, slug},
    "period": {start: "2026-06-12T00:00:00Z", end: "2026-06-13T00:00:00Z"},
    "counters": {
      "observations": 1234,
      "agents": 5,
      "agent_runs_today": 42,
      "flow_runs_today": 7,
      "cost_usd_today": 12.34,
      "tokens_in_today": 500000,
      "tokens_out_today": 80000
    },
    "limits": {
      "rate_limit_per_minute": 1000,
      "max_flow_duration_seconds": 300
    }
  }
Y los counters reflejan el día UTC actual
```

### Escenario 2: `GET /api/v1/usage/history?days=30` retorna histórico

```gherkin
Dado que quiero ver el último mes de uso
Cuando hago `GET /api/v1/usage/history?days=30`
Entonces el response es 200 con:
  {
    "organization": {...},
    "history": [
      {date: "2026-06-12", observations: 1234, cost_usd: 12.34, agent_runs: 42, flow_runs: 7},
      {date: "2026-06-11", observations: 1100, cost_usd: 8.50, agent_runs: 38, flow_runs: 5},
      ...
      {date: "2026-05-13", ...}  // 30 días atrás
    ]
  }
Y limit: days <= 365 (no se puede pedir 10 años)
Y default days=7 si no se pasa
```

### Escenario 3: Endpoint es read-only

```gherkin
Dado que el endpoint es GET
Cuando intento POST, PUT, DELETE
Entonces 405 Method Not Allowed
Y NUNCA modifica estado (ni counters, ni limits, ni nada)
```

### Escenario 4: Solo ve SU org, no otras

```gherkin
Dado que estoy autenticado como user de org A
Cuando hago GET /usage/current
Entonces solo veo counters de org A
Y NUNCA veo datos de org B (filtro por principal.organization_id)
Y si por bug se filtra data de B → test e2e FALLA
```

### Escenario 5: Performance aceptable

```gherkin
Dado que la org tiene 1M de cost_logs en el último mes
Cuando hago GET /usage/history?days=30
Entonces el endpoint responde en <500ms
Y la query usa índices pre-calculados o agregaciones
  (no escanea full table)
```

### Escenario 6: Sabotaje — endpoint expone data de TODAS las orgs

```gherkin
Dado que el código tiene un bug (sabotaje) que no filtra por org_id
Y user de org A hace GET /usage/current
Entonces el response incluye datos de org B, C, D (mezclados)
Y el test e2e que assserta "solo data de la propia org" DEBE FALLAR
Cuando restauro el filtro WHERE organization_id = $principal
Entonces el test verde
```

### Escenario 7: Edge case — org sin uso

```gherkin
Dado que la org es nueva (0 observaciones, 0 runs, 0 costo)
Cuando hago GET /usage/current
Entonces response 200 con todos los counters en 0
Y el period es el día UTC actual
Y NUNCA retorna 404 (una org sin uso sigue existiendo)
```

### Escenario 8: Edge case — día parcial

```gherkin
Dado que la org tiene 5 horas de uso hoy (00:00-05:00 UTC)
Cuando hago GET /usage/current a las 14:00 UTC
Entonces el period.start = 2026-06-12T00:00:00Z
Y el period.end = 2026-06-13T00:00:00Z
Y los counters reflejan el día completo hasta el momento (5h
  de uso + 9h sin uso = solo se cuentan los que hubo)
```

## Notas

- Los datos de `cost_logs`, `observations`, `agent_runs`, `flow_runs`
  YA EXISTEN. Solo se agrega la capa de API que los agrega y los
  sirve.
- La query puede ser cara con muchos datos. Estrategia:
  - Para `usage/current`: query directa con índice `(organization_id,
    created_at)`.
  - Para `usage/history`: agregación pre-calculada en una vista
    materializada o tabla `usage_daily_aggregates` refrescada por
    el job de cost (issue-15.3).
- NO hay rate limit específico para estos endpoints — usan el
  per-org general (33.1). Si se vuelven un problema, agregar
  rate limit por endpoint en el futuro.
- NO es comercial (no se factura). Es VISIBILIDAD para el
  cliente.
