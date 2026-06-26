# Cron: usos propuestos

## Estado actual (verificado)

La infraestructura de cron de domain-mcp está **100% implementada pero inerte**:

- Tabla `crons` (mig `000016`) + `cron_executions` (mig `000084`): schedule, `target_type ∈ {flow, agent, skill}`, `target_id`, `inputs` JSONB, `enabled`, `next_run_at`.
- Scheduler (`internal/scheduler/cron/scheduler.go`): poll cada 30s, leader election, `SELECT ... FOR UPDATE SKIP LOCKED`, timeout 10 min por ejecución, anti-solapamiento (`skipped_overlap`).
- Dispatcher unificado (`internal/dispatch/dispatcher.go`): rutea a `RunFlow`/`RunAgent`/`RunSkill`, con auditoría y métricas.
- Tools MCP: `domain_cron_create`, `domain_cron_set_enabled`, `domain_cron_delete`, `domain_cron_history`, `domain_cron_list`.
- UI en domain-admin (`app/maintainers/crons/`): CRUD + toggle + export CSV.

**No hay ningún cron seedeado ni en uso.** El motor está encendido esperando carga.

> Nota: el dispatcher despacha `flow`/`agent`/`skill` **server-side**. Las fases SDD que mutan el workspace (p. ej. `sdd-apply`) corren en el cliente IDE, no acá. Por eso los usos de cron viables son los que **no requieren tocar el workspace**: lectura, agregación, alertas y disparo de flows que ya estén pensados para correr sin cliente (Solo/Async).

---

## Candidatos (con tradeoffs)

### 1. Review de cumplimiento nightly (recomendado) — conecta con `sdd-review`

**Qué**: cron diario que dispara la lógica del revisor de implementación (fase `sdd-review`, recién agregada) sobre proyectos con specs cerradas en las últimas 24 h, re-evaluando el cumplimiento de políticas/skills y dejando un checkpoint `tdd_verifications` con `kind='policy_review'`.

**Target**: `skill` o `flow` que envuelva la lógica de review en modo Solo/Async (sin cliente IDE).

**Pro**: da uso concreto al motor inerte y cierra el bucle con el revisor; detecta drift de cumplimiento aunque el cierre original haya pasado el gate. **Contra**: el review profundo necesita LLM + acceso al diff; en cron sólo es viable una versión read-only que consulte estado persistido (no re-corre tests). Empezar con alcance "resumen de violations abiertas", no "re-análisis completo".

### 2. Auditoría de `tdd_verifications` colgadas

**Qué**: cron que marca como `failed` los checkpoints en `running`/`pending` con `started_at` viejo (> N horas) y emite un resumen de los `failed` recientes.

**Target**: `skill` read/write acotada sobre `tdd_verifications`.

**Pro**: barato, determinista, sin LLM; limpia ruido que ya existe. **Contra**: utilidad baja si el volumen de checkpoints es chico.

### 3. Digest de salud del MCP

**Qué**: cron que agrega los `mcp_health_checks` (mig `000174`) y los `cron_executions` recientes en un resumen (uptime, fallos, latencias) y lo publica a un canal/knowledge_doc.

**Target**: `skill` de agregación + notificación.

**Pro**: observabilidad operativa real; reusa tablas existentes. **Contra**: requiere definir el canal de salida (webhook/notificación); cardinalidad de métricas a cuidar (ver policy `low-cardinality-metrics`).

### 4. Housekeeping de soft-deletes

**Qué**: cron semanal que purga físicamente filas con `deleted_at` anterior a una retención (p. ej. 90 días) en tablas con soft-delete.

**Target**: `skill` de mantenimiento.

**Pro**: controla crecimiento de tablas; totalmente desacoplado del revisor. **Contra**: destructivo — exige whitelist explícita de tablas y dry-run previo; cuidado con FKs.

### 5. Reindex de embeddings pendientes

**Qué**: cron que detecta `knowledge_chunks`/`skills` sin `embedding` y los encola para reindexar.

**Target**: `flow` o `skill` que invoque el proveedor de embeddings.

**Pro**: mantiene la búsqueda semántica fresca sin intervención. **Contra**: costo de API por corrida; necesita rate-limiting y batch.

---

## Recomendación

Arrancar con **#1 (review nightly)** porque cierra el bucle con el revisor de implementación y justifica el motor de cron con un caso de valor inmediato, más **#2** como complemento barato y determinista. Diferir #4/#5 hasta tener señal de que el volumen lo amerita (YAGNI).

Cualquiera se crea sin tocar código nuevo del scheduler: sólo `domain_cron_create` con el `target_type`/`target_id` correspondiente.
