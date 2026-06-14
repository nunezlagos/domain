# issue-34.2-org-delete-gdpr-cascade

**Origen:** `REQ-34-saas-operations`
**Prioridad tentativa:** media
**Tipo:** feature (GDPR/operational)

## Historia de usuario

**Como** operador del VPS multi-tenant
**Quiero** tener un comando admin para borrar completamente una org y todos sus datos
**Para** cumplir con GDPR right-to-be-forgotten cuando un cliente lo pide, o limpiar cuentas abandonadas

## Criterios de aceptación

### Escenario 1: Delete via CLI

```gherkin
Dado que existe la org "acme-corp" con 1000 observations, 50 agents, etc.
Cuando corro `domain org delete acme-corp --confirm` desde la máquina con DOMAIN_DATABASE_AUTH_URL
Entonces el comando:
  1. Verifica que la org existe
  2. Pide doble confirm (a menos que --yes)
  3. Audit log entry ANTES del delete: action=org.delete_initiated, resource=org/<id>, metadata={slug, observation_count, agent_count, ...}
  4. DELETE FROM organizations WHERE id = $1 (CASCADE borra todo)
  5. Audit log entry DESPUÉS: action=org.delete_completed, duration_ms, rows_deleted_per_table
Y exit 0
Y la org ya no existe (ni en DB ni en S3)
```

### Escenario 2: Delete via API admin

```gherkin
Dado que soy admin autenticado
Cuando hago `DELETE /api/v1/admin/orgs/{id}` con header `X-Confirm: true`
Entonces el server:
  1. Verifica el header `X-Confirm: true` (defensa contra click accidental)
  2. Audit log antes
  3. CASCADE delete
  4. Audit log después
  5. Limpia S3 prefix de la org
Y response 204 No Content
Y la org ya no existe
```

### Escenario 3: ON DELETE CASCADE funciona

```gherkin
Dado que la org tiene datos en: observations, prompts, knowledge_docs, skills, agents, flows, flow_runs, cost_logs, audit_log, etc.
Cuando se hace DELETE FROM organizations WHERE id = $1
Entonces TODAS las filas relacionadas se borran (CASCADE)
Y NO quedan foreign key violations
Y el conteo final es 0 filas para esa org
```

### Escenario 4: S3 prefix de la org se limpia

```gherkin
Dado que la org tiene attachments en S3 bajo `orgs/<org-id>/*`
Cuando se completa el CASCADE delete
Entonces el server hace `s3 rm --recursive s3://<bucket>/orgs/<org-id>/`
Y S3 retorna éxito
Y los attachments ya no existen
Y si S3 falla, el delete de DB se completa igual pero el audit log marca "s3_cleanup_failed: true" + warning
```

### Escenario 5: Idempotente

```gherkin
Dado que la org "acme-corp" ya fue borrada
Cuando corro `domain org delete acme-corp --confirm` de nuevo
Entonces el comando:
  - Detecta que la org no existe
  - Imprime "org acme-corp not found (skipping)"
  - Exit 0 (no error)
Y NO intenta hacer delete de nuevo
Y NO inserta audit log (no hay nada que auditar)
```

### Escenario 6: Doble confirm para evitar accidents

```gherkin
Dado que corro `domain org delete acme-corp` SIN --confirm
Cuando el comando se ejecuta
Entonces imprime el impacto esperado:
  "ABOUT TO DELETE org 'acme-corp' (id=uuid):
   - 1000 observations
   - 50 agents
   - 200 flow_runs
   - 50 cost_logs
   - 500 audit_log entries
   - 10 attachments in S3
   Proceed? Type 'DELETE acme-corp' to confirm:"
Si el user tipea exactamente "DELETE acme-corp" → procede
Si tipea otra cosa o Enter → abort con exit 1
```

### Escenario 7: Sabotaje — delete sin confirm

```gherkin
Dado que el código tiene un bug (sabotaje) que skipea el confirm prompt
Y user corre `domain org delete acme-corp` accidentalmente
Entonces la org se borra sin pedir confirmación
Y el test e2e que assserta "delete pide doble confirm sin --confirm"
DEBE FALLAR
Cuando restauro el confirm prompt
Entonces el test verde
```

### Escenario 8: Edge case — org con datos en retention (audit_log)

```gherkin
Dado que la org tiene 5000 entries en audit_log (algunos con
occurred_at < 90 días, otros más recientes)
Cuando se hace delete
Entonces TODOS los audit_log entries de la org se borran
(incluso los históricos con retention > 90 días)
Y NO se respeta la retention para el delete (es GDPR, no
operacional)
```

## Notas

- Las foreign keys YA están con `ON DELETE CASCADE` (ver
  migrations históricas). El issue es el wrapping operacional
  (audit, confirm, S3 cleanup, idempotencia).
- El audit log entry ANTES del delete es crítico para
  forense: si algo sale mal, sabemos QUÉ se iba a borrar.
- S3 cleanup es best-effort: si falla, el delete de DB igual
  procede (GDPR es prioridad sobre attachments).
- El endpoint admin requiere auth de admin (no Bearer normal).
