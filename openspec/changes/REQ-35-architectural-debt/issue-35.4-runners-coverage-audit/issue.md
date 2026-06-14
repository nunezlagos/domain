# issue-35.4-runners-coverage-audit

**Origen:** `REQ-35-architectural-debt`
**Prioridad tentativa:** baja
**Tipo:** chore (research + decisión)

## Historia de usuario

**Como** tech lead de domain
**Quiero** saber si los runners server-side (agent runner, flow runner) se usan realmente en producción
**Para** decidir si vale la pena mantenerlos, simplificarlos, o marcarlos como "beta — usá MCP directo" basándome en datos, no en especulación

## Criterios de aceptación

### Escenario 1: Reporte de uso real de los últimos 30 días

```gherkin
Dado que el server está en producción hace 30+ días
Cuando corro `make audit-runners-usage` (o `domain admin runners-usage --days=30`)
Entonces el comando genera un reporte con:
  - agent_runner: cuántos agent_runs se ejecutaron, % success, % failed, avg duration, top 5 agents más usados
  - flow_runner: cuántos flow_runs, % success/failed, avg duration, top 5 flows
  - skill_runner (server-side): cuántos skill_executions, % success/failed
  - Distribución por source: cuántos disparados via MCP, cron, webhook
  - Por org: top 10 orgs por uso
Y el reporte se imprime como tabla ASCII y también se exporta a
`reports/runners-usage-<date>.json`
```

### Escenario 2: Identificar runners no usados

```gherkin
Dado que el reporte está generado
Cuando lo analizamos
Entonces podemos categorizar:
  - USADO: >= 10 ejecuciones/mes
  - POCO USADO: 1-9 ejecuciones/mes
  - NUNCA USADO: 0 ejecuciones en 30 días
Y el reporte marca explícitamente los runners "NUNCA USADO"
con un WARNING al final
```

### Escenario 3: Decisión basada en datos

```gherkin
Dado que el reporte muestra uso real
Cuando escribimos el ADR / decisión
Entonces el documento tiene:
  - "Datos: agent_runner = 50 ejecuciones/mes (USADO), flow_runner = 200/mes (USADO), skill_runner = 0/mes (POCO USADO)"
  - Decisión por cada runner:
    - USADO: mantener, priorizar bugs/features acá
    - POCO USADO: mantener pero documentar como beta
    - NUNCA USADO: marcar como "beta — usá MCP directo" en docs, o mover a REQ deferido
Y la decisión es REVERSIBLE (si en 6 meses el uso sube, se revisa)
```

### Escenario 4: Telemetría adicional (correlación con LLM usage)

```gherkin
Dado que tenemos datos de cost_logs (issue-15.3) y de runner usage
Cuando cruzamos
Entonces podemos responder:
  - ¿Los flows server-side usan LLM directamente, o delegan al
    MCP client que ya tiene LLM?
  - ¿Cuánto cost extra genera el flow runner vs hacer lo mismo
    via MCP?
  - Si el cost es comparable → mantener server-side (latency,
    offline).
  - Si el cost es 2x → reconsiderar (recomendar MCP directo).
```

### Escenario 5: Top agents/flows con feedback negativo

```gherkin
Dado que un user reporta "el flow X siempre falla"
Cuando cruzamos runner usage con audit_log
Entonces podemos ver:
  - Cuántas veces corrió X
  - Cuántas fallaron
  - Si hay un patrón (e.g. siempre falla con el mismo input)
Y el reporte incluye "top 5 agents/flows con >50% failure rate"
```

### Escenario 6: Output compartido con el equipo

```gherkin
Dado que el reporte se genera
Cuando termina
Entonces:
  - Imprime summary en stdout
  - Guarda JSON detallado en reports/
  - (Opcional) envía email al admin con el summary
Y el archivo es commiteable a git (no contiene PII — solo
counts y promedios)
```

### Escenario 7: Sabotaje — reporte miente sobre uso

```gherkin
Dado que el código del reporte tiene un bug (sabotaje) que hardcodea
"USADO" para todos los runners (sin chequear el threshold)
Cuando corro el comando
Entonces el reporte marca TODO como USADO, sin importar el uso real
Y el test e2e que assserta "runner con 0 ejecuciones es marcado
como NUNCA USADO" DEBE FALLAR
Cuando restauro la lógica de threshold check
Entonces el test verde
```

### Escenario 8: Edge case — server tiene <30 días de datos

```gherkin
Dado que el server tiene solo 5 días de telemetría
Cuando corro el comando
Entonces el reporte dice: "5 days of data available (recommend 30+ days for accurate analysis)"
Y los thresholds de "USADO" se ajustan proporcionalmente (e.g.
"USADO" = >= 1 ejecución, no >= 10)
Y exit 0 (no falla por datos insuficientes)
```

## Notas

- El output es DECISIONAL, no de enforcement. Es el input para
  el ADR / decisión del 35.2 (skills) y 35.1 (dispatcher).
- El reporte NO debe contener datos sensibles (org names, user
  emails) — solo counts y promedios. Esto lo hace commiteable.
- Es UN comando, no un servicio corriendo. Se corre a demanda.
- Si los datos muestran que los runners NUNCA se usan, eso NO
  es motivo para borrarlos inmediatamente. Es motivo para
  ETIQUETARLOS como "beta" y re-evaluar.
