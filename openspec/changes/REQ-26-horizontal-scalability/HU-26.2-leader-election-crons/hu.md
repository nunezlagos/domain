# HU-26.2-leader-election-crons

**Origen:** `REQ-26-horizontal-scalability`
**Persona:** platform-engineer, security-officer
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** plataforma con N pods replicados
**Quiero** leader election para crons singleton (backup, drift, password rotation, slow query analyzer, etc.)
**Para** que el cron NO ejecute N veces en paralelo (1 por pod)

## Mecanismo

Postgres advisory lock per cron name. Solo el pod que adquiere el lock ejecuta. TTL implícito = vida de la conexión + watchdog.

## Criterios de aceptación

### Escenario 1: 5 pods, 1 cron, 1 ejecución

```gherkin
Dado que existen 5 pods Domain
Y cron `slow-query-analyzer` programado cada 5min
Cuando llega el tick
Entonces los 5 pods intentan `pg_try_advisory_lock(hash('slow-query-analyzer'))`
Y exactamente 1 lo obtiene
Y solo ese 1 ejecuta el cron
Y los otros 4 logean "skipped, leader elected elsewhere"
```

### Escenario 2: Leader cae mid-execution

```gherkin
Dado que pod-leader cae durante cron
Cuando la conexión se cierra
Entonces el advisory lock se libera automáticamente
Y siguiente tick: otro pod adquiere el lock + ejecuta
Y métrica `domain_cron_leader_changes_total` incrementa
```

### Escenario 3: Worker heartbeat watchdog

```gherkin
Dado que existe sistema con cron periódico cada 60s
Y leader actualiza `system_crons.last_heartbeat_at` cada 10s mientras corre
Cuando otro pod inspecciona stale lock (heartbeat >120s sin update)
Entonces puede forzar nuevo leader via `pg_advisory_unlock` + re-elect
Y se logea audit "cron.leader.forced_takeover"
```

### Escenario 4: Métricas observables

```gherkin
Dado que la métrica `domain_cron_leader{cron,pod}` exists (gauge 0/1)
Cuando query Prometheus
Entonces vemos exactamente 1 pod con valor 1 por cron en cualquier instant
Y alerta si 0 pods con valor 1 por >5min (no leader = crons no corren)
```

## Análisis breve

- **Qué pide:** advisory lock + watchdog + métricas + audit
- **Esfuerzo:** S
- **Riesgos:** split-brain si network partition; aceptable porque Postgres es single source
