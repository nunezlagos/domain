# issue-08.12-orphan-runs-audit-cron

**Origen:** `REQ-08-agent-system`
**Prioridad tentativa:** media
**Tipo:** infrastructure
**Bloquea a:** `issue-08.10-sdd-pipeline-orchestrator`
**RFC:** `docs/rfc/0006-sdd-pipeline-orchestrator.md` (ADR-2 enforcement híbrido)

## Tarea técnica

**Como** plataforma Domain
**Quiero** un system cron diario que cuente `agent_runs` con `flow_run_id IS NULL` creados sin la flag `standalone` (es decir, bypaseando el service-layer enforcement) e incremente la métrica `domain_agent_runs_orphan_total`
**Para** que el enforcement híbrido del orquestador (issue-08.10 ADR-2) tenga visibility cuando alguien hace INSERT directo en BD bypaseando el service Go

## Modelo

- **System cron diario** (NO user-defined): `internal/scheduler/cron/system/orphan_runs_audit.go`
- **Tick:** 1x al día (configurable, default `0 4 * * *` = 4am UTC) — ventana baja-actividad
- **Detección:** `agent_runs` con `flow_run_id IS NULL` AND `metadata->>'standalone' IS NULL` AND `created_at > last_ack_at`
- **Acción:** cuenta + incrementa métrica `domain_agent_runs_orphan_total{org_id, reason}` con reason='bypass_service_layer'; persiste `last_ack_at` para no double-count
- **Métrica:** counter ya definido en issue-08.10 (`internal/metrics/agent.go`)
- **Cero schema BD nuevo** — `agent_runs.metadata JSONB` ya existe

## Criterios de aceptación

### Escenario 1: Detección de bypass

```gherkin
Dado que alguien ejecuta INSERT INTO agent_runs (id, agent_id, flow_run_id, metadata, ...)
  VALUES (gen_random_uuid(), <agent_id>, NULL, '{}', ...) via pool directo (no service)
Cuando el cron orphan_runs_audit corre su tick diario
Entonces cuenta el INSERT como orphan (flow_run_id IS NULL AND metadata->>'standalone' IS NULL)
Y incrementa domain_agent_runs_orphan_total{org_id, reason='bypass_service_layer'} en 1
```

### Escenario 2: standalone=true NO se cuenta

```gherkin
Dado que el service crea agent_runs con WithStandalone(true)
Y persiste agent_runs.metadata = {"standalone": true, "reason": "debug"}
Cuando el cron corre
Entonces NO cuenta como orphan
Y domain_agent_runs_standalone_total{org_id, reason} se incrementa por el service-layer (NO el cron)
```

### Escenario 3: Idempotencia via last_ack_at

```gherkin
Dado que el cron ya procesó hasta last_ack_at='2026-06-09 04:00:00'
Y los orphans anteriores ya fueron contados (no se cuentan otra vez)
Cuando el cron corre el día siguiente
Entonces sólo considera agent_runs con created_at > '2026-06-09 04:00:00'
Y actualiza last_ack_at al timestamp del último procesado
Y NO double-count de orphans anteriores
```

### Escenario 4: Alert dispara si orphans > 0

```gherkin
Dado que el cron cuenta 1+ orphans en el último día
Cuando expone la métrica
Entonces AlertManager dispara alert 'AgentRunsOrphanDetected' con severity='warning'
Y notifica via canal configurado (issue-20.x)
```

### Escenario 5: Leader election en HA

```gherkin
Dado que hay 3 instancias del server
Cuando llega el tick diario
Entonces SÓLO el leader ejecuta la query de conteo
Y los otros 2 NO ejecutan (evita double-count)
```

### Escenario 6: Sabotage — bypaseo intencional

```gherkin
Dado que un test ejecuta INSERT INTO agent_runs (flow_run_id=NULL, metadata='{}', ...) en setup
Cuando el cron corre dentro de 24h
Entonces la métrica orphan_total se incrementa en 1
Y el test sabotage valida que el cron lo atrapó
```

## Análisis breve

- **Qué pide:** cron diario + count query + métrica + persist ack timestamp + alert
- **Módulos:** `internal/scheduler/cron/system/orphan_runs_audit.go` (NUEVO), métrica ya definida en issue-08.10
- **Esfuerzo:** S (1-2h con tests)
- **Riesgos:** false positives si `standalone` flag se escribe distinto — definir contract estricto en metadata
- **Cero schema BD:** todo en columnas existentes (`metadata`, `created_at`) + 1 row de control en tabla `system_state` (o equivalente)
