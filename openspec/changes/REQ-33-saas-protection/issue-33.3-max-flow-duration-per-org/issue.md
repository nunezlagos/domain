# issue-33.3-max-flow-duration-per-org

**Origen:** `REQ-33-saas-protection`
**Prioridad tentativa:** media
**Tipo:** feature (operational)

## Historia de usuario

**Como** operador del VPS multi-tenant
**Quiero** que los flow_runs de cada org tengan un timeout máximo configurable
**Para** que un cliente con un flow que entra en loop infinito no bloquee las goroutines del scheduler compartido y afecte a otros clientes

## Criterios de aceptación

### Escenario 1: Flow excede timeout → cancela

```gherkin
Dado que org A tiene `max_flow_duration_seconds = 300` (5min)
Y un flow_run de org A lleva 350 segundos corriendo
Cuando el scheduler detecta el overrun
Entonces el flow_run se marca como `failed` con `error_code: "max_duration_exceeded"`
Y el log dice: "flow_run <id> for org <name> exceeded max duration 300s (was 350s); cancelling"
Y las goroutines del flow se cancelan (context cancel propagado)
```

### Escenario 2: Flow dentro del budget → corre normal

```gherkin
Dado que org A tiene `max_flow_duration_seconds = 300`
Y un flow_run tarda 200 segundos en completarse
Cuando termina OK
Entonces el flow_run se marca como `succeeded` normal
Y NO se dispara la cancelación
```

### Escenario 3: Default 5min, configurable per-org

```gherkin
Dado que una org nueva sin config explícita
Cuando crea un flow_run
Entonces el budget es 300s (5 min, default)
Y se loggea "org X using default max_flow_duration 300s" solo la primera vez
```

### Escenario 4: Tabla de config

```gherkin
Dado que `org_flow_config` es la tabla de config per-org para flow
Y tiene columna `max_flow_duration_seconds INT NOT NULL DEFAULT 300`
Y la migración la crea con default 300 para todas las orgs existentes
```

### Escenario 5: Hook en el flow runner

```gherkin
Dado que el `flowrunner.Runner` (cmd/domain/main.go) ya tiene
`RunRecovery` con `StaleAfter: 5min` (issue-09.6)
Cuando flow runs se marcan stale
Entonces además de marcarlos failed, también verifica el
`max_flow_duration_seconds` per-org
Y si el flow tiene un budget menor que el stale threshold, usa el menor
```

### Escenario 6: Edge case — flow con sub-flows anidados

```gherkin
Dado que un flow_run tiene 3 sub-flows anidados
Y el total acumulado es 350s
Y el budget es 300s
Cuando se detecta el overrun
Entonces TODOS los sub-flows se cancelan (no solo el top-level)
Y el context cancel se propaga recursivamente
```

### Escenario 7: Sabotaje — max_flow_duration no se aplica

```gherkin
Dado que el código tiene un bug (sabotaje) que ignora el per-org
max_flow_duration y siempre usa el default 5min
Y una org tiene config 60s (cliente que paga poco, quiere
cortar flows rápido)
Y un flow_run tarda 200s
Cuando el scheduler revisa
Entonces NO se cancela (sabotaje: usa default 5min en vez de 60s)
Y el test e2e que assserta "flow de 200s en org con budget 60s se
cancela" DEBE FALLAR
Cuando restauro el lookup per-org
Entonces el test verde
```

### Escenario 8: Métrica + alerta

```gherkin
Dado que un flow_run fue cancelado por max_duration
Cuando se registra
Entonces `metrics.FlowRunCancelledByMaxDuration.Inc()` se incrementa
Y un log estructurado se emite con `org_id`, `flow_run_id`,
`duration_seconds`, `budget_seconds`
Y opcionalmente el admin endpoint lista los últimos N cancelados
```

## Notas

- El `flowrunner.Runner` YA TIENE cancelación por context. Solo
  hay que pasarle el `max_flow_duration` correcto.
- El job `RunRecovery` YA EXISTE (issue-09.6) y marca stale runs
  como failed. La feature es: además de stale, también respeta
  el per-org budget.
- NO es "premium tier" — todos los orgs tienen el mismo default,
  configurables individualmente.
