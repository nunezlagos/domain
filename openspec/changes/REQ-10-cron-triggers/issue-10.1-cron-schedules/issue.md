# issue-10.1-cron-schedules

**Origen:** `REQ-10-cron-triggers`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** ingeniero de automatización
**Quiero** definir schedules cron que ejecuten un flow o agente automáticamente en un intervalo, con soporte de timezone y registro de ejecuciones
**Para** automatizar tareas periódicas como reportes diarios, limpieza de datos o monitoreo

## Criterios de aceptación

### Escenario 1: Crear un schedule cron

```gherkin
Dado que soy un usuario autenticado con permiso `cron:write`
Cuando envío un POST a `/api/v1/crons` con el cuerpo:
  """
  {
    "name": "Daily Report",
    "cron_expression": "0 8 * * 1-5",
    "flow_slug": "generate-report",
    "project_id": "proj-abc-123",
    "timezone": "America/Argentina/Buenos_Aires",
    "enabled": true,
    "params": {"format": "pdf", "recipients": ["ops@example.com"]}
  }
  """
Entonces el sistema responde con HTTP 201
Y el body contiene `id` (UUID)
Y el body contiene `next_run` calculado: próximo día hábil a las 08:00 AR-TZ
Y `cron_expression` es "0 8 * * 1-5"
```

### Escenario 2: Crear schedule que ejecuta un agente en vez de un flow

```gherkin
Dado que existe un agente con slug "health-checker"
Cuando creo un cron con `agent_slug: "health-checker"` (sin flow_slug)
Entonces el sistema acepta el schedule
Y next_run se calcula correctamente

Dado que creo un cron sin flow_slug ni agent_slug
Entonces el sistema responde con HTTP 422
Y el error indica que debe especificar flow_slug o agent_slug

Dado que creo un cron con ambos flow_slug y agent_slug
Entonces el sistema responde con HTTP 422
Y el error indica que solo uno debe ser especificado
```

### Escenario 3: Scheduler evalúa cada minuto y ejecuta schedules debidos

```gherkin
Dado que existen crons habilitados con diferentes schedules
Y el scheduler se ejecuta cada 60 segundos
Cuando el scheduler evalúa a las 08:00:00
Entonces encuentra todos los crons cuya next_run ≤ 08:00:00
Y para cada cron, inicia una ejecución del flow/agente asociado
Y actualiza `last_run` al timestamp actual
Y recalcula `next_run` basado en la cron_expression
Y registra un nuevo registro en la tabla de ejecución histórica
```

### Escenario 4: Cron deshabilitado no se ejecuta

```gherkin
Dado que existe un cron con `enabled: false`
Cuando el scheduler evalúa los schedules debidos
Entonces el cron deshabilitado NO se incluye en la evaluación
Y su next_run no se actualiza

Cuando envío un PATCH a `/api/v1/crons/{id}` con `{"enabled": true}`
Entonces el cron se habilita
Y next_run se recalcula inmediatamente
```

### Escenario 5: Timezone afecta el cálculo de next_run

```gherkin
Dado que creo un cron con `cron_expression: "0 9 * * *"` y `timezone: "America/New_York"`
Y otro cron con misma expresión y `timezone: "Asia/Tokyo"`
Cuando el scheduler evalúa a las 09:00 NY
Entonces solo el cron NY se ejecuta
Y el cron Tokyo se ejecutará 13h después (diferencia horaria)

Dado que no especifico timezone
Entonces se usa UTC por defecto
```

### Escenario 6: Historial de ejecuciones

```gherkin
Dado que un cron se ha ejecutado 5 veces
Cuando envío un GET a `/api/v1/crons/{id}/history?limit=3`
Entonces recibo los últimos 3 registros de ejecución
  """
  {
    "data": [
      {"scheduled_at": "...", "executed_at": "...", "status": "completed", "flow_run_id": "..."},
      {"scheduled_at": "...", "executed_at": "...", "status": "failed", "flow_run_id": "...", "error": "..."},
      {"scheduled_at": "...", "executed_at": "...", "status": "completed", "flow_run_id": "..."}
    ],
    "pagination": {"total": 5, "limit": 3, "offset": 0}
  }
  """
```

### Escenario 7: Editar y eliminar schedule

```gherkin
Dado que existe un cron con id "cron-abc"
Cuando envío un PUT a `/api/v1/crons/cron-abc` con nueva `cron_expression`
Entonces el sistema actualiza el cron
Y next_run se recalcula con la nueva expresión

Cuando envío un DELETE a `/api/v1/crons/cron-abc`
Entonces el sistema responde con HTTP 204
Y el cron ya no aparece en el listado
```

### Escenario 8: Cron expression inválida

```gherkin
Dado que intento crear un cron con `cron_expression: "invalid"`
Entonces el sistema responde con HTTP 422
Y el error indica que la expresión cron es inválida

Dado que intento crear un cron con `cron_expression: "*/5 * * * *"`
Y `timezone: "Invalid/Timezone"`
Entonces el sistema responde con HTTP 422
Y el error indica que el timezone es inválido
```

## Análisis breve

- **Qué pide realmente:** CRUD de cron schedules, scheduler worker que evalúa cada minuto y ejecuta flows/agentes, timezone support, histórico de ejecuciones.
- **Módulos sospechados:** `internal/cron/`, `internal/scheduler/scheduler.go`, `internal/api/handlers/cron.go`, `internal/models/cron.go`
- **Riesgos / dependencias:** Depende de REQ-08 (agentes) y REQ-09 (flows) para ejecución. El scheduler debe manejar overlaps (si un cron tarda más que su intervalo).
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
