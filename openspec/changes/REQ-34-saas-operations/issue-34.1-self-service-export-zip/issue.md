# issue-34.1-self-service-export-zip

**Origen:** `REQ-34-saas-operations`
**Prioridad tentativa:** media
**Tipo:** feature (GDPR/portability)

## Historia de usuario

**Como** usuario de una org en domain (cliente SaaS)
**Quiero** poder descargar un ZIP con TODOS mis datos del sistema (observations, prompts, knowledge, configs)
**Para** tener una copia de seguridad personal, migrar a otro sistema, o cumplir con GDPR data portability

## Criterios de aceptación

### Escenario 1: Export devuelve ZIP con todos los datos de la org

```gherkin
Dado que estoy autenticado (API key o sesión) como user de org A
Cuando hago `GET /api/v1/export`
Entonces el response es 200 con:
  - Content-Type: application/zip
  - Content-Disposition: attachment; filename="domain-export-<org-slug>-<date>.zip"
Y el ZIP contiene:
  - observations.jsonl.gz: cada observation como JSON line comprimido
  - prompts.jsonl.gz
  - knowledge_docs.jsonl.gz
  - skills.jsonl.gz
  - agents.jsonl.gz
  - flows.jsonl.gz
  - audit_log.jsonl.gz (solo eventos del user caller, no de toda la org — privacidad)
  - metadata.json: {org_id, org_slug, exported_at, exported_by_user_id, schema_version}
Y el stream se envía directamente (NO se almacena en S3 ni en el server)
Y el audit_log tiene entry: actor=<user>, action=export, resource=org/<id>
```

### Escenario 2: Solo datos de la propia org

```gherkin
Dado que user de org A hace export
Y org B tiene 1000 observations
Cuando A descarga el ZIP
Entonces el ZIP contiene SOLO las observations de A
Y NUNCA contiene datos de B
Y si por bug se filtra data de B → test e2e FALLA
```

### Escenario 3: Stream no agota memoria

```gherkin
Dado que la org tiene 10M de observations (~5GB de JSON sin comprimir)
Cuando hago export
Entonces el server NO carga todo en memoria
Y streamea desde Postgres → gzip → zip en chunks
Y el peak memory del server durante el export es <200MB
Y el cliente puede empezar a descargar inmediatamente (chunked transfer)
```

### Escenario 4: Edge case — org sin datos

```gherkin
Dado que la org es nueva (0 observations, 0 prompts, etc.)
Cuando hago export
Entonces response 200 con ZIP válido
Y contiene SOLO metadata.json
Y los .jsonl.gz están presentes pero vacíos (0 bytes, gzip válido)
Y el header Content-Length coincide con el tamaño real
```

### Escenario 5: Edge case — datos muy grandes (timeouts)

```gherkin
Dado que la org tiene 50M de observations (export tomaría 10 minutos)
Cuando hago export
Entonces el server NO cierra la conexión por timeout (configurar
ReadTimeout/WriteTimeout del http.Server a >30min para este endpoint, o
deshabilitar el timeout solo para /export)
Y el cliente puede usar `curl --max-time 1800` o similar
Y si el cliente cierra la conexión, el server libera los recursos
```

### Escenario 6: Rate limit NO se aplica (es legítimo, no abuse)

```gherkin
Dado que el rate limit per-org es 1000/min (33.1)
Y un user hace export de 10GB
Cuando el server streamea
Entonces NO se cuenta contra el rate limit (es una operación larga legítima)
Y el endpoint tiene allowlist en el rate limit middleware
Y se loggea el export con `duration_seconds`, `bytes_streamed`
```

### Escenario 7: Sabotaje — export incluye datos de otras orgs

```gherkin
Dado que el código tiene un bug (sabotaje) que NO filtra por organization_id
Y user de org A hace export
Entonces el ZIP contiene data de B, C, D (mezclado)
Y el test e2e que assserta "solo data de A" DEBE FALLAR
Cuando restauro el filtro WHERE organization_id = $principal
Entonces el test verde
```

### Escenario 8: Audit log registra la acción

```gherkin
Dado que un user de org A hace export
Cuando termina
Entonces hay un entry en audit_log:
  {
    actor_user_id: <user>,
    organization_id: <org A>,
    action: "export",
    resource: "org/<id>",
    metadata: {bytes_streamed: 12345, duration_ms: 3000, format: "zip-jsonl-gz"},
    occurred_at: <now>
  }
Y el admin puede verlo en /admin/audit (34.5)
```

## Notas

- El formato es JSON Lines (.jsonl) para que sea fácil de parsear
  streaming. gzip por archivo. NO usar CSV (perdemos tipos).
- `metadata.json` es clave: el schema del DB puede cambiar entre
  versiones. El metadata documenta la versión para que un
  importador sepa qué esperar.
- NO se provee endpoint de import (decisión explícita del
  user). El ZIP es para que el usuario decida qué hacer.
- Si en el futuro se quiere, el formato se extiende con
  `flows.jsonl.gz` y `attachments/` (binarios).
