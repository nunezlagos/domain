# HU-23.2-soft-delete-restore

**Origen:** `REQ-23-data-lifecycle`
**Persona:** org-member, org-admin
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario
**Quiero** que el delete sea soft con papelera y restore por TTL
**Para** recuperar ítems borrados por error sin involucrar a soporte

## Criterios de aceptación

### Escenario 1: Soft-delete uniforme

```gherkin
Dado que existen entidades con `deleted_at TIMESTAMPTZ`
Cuando hago DELETE /api/v1/observations/:id
Entonces `deleted_at = NOW()`
Y queries normales no la devuelven
Y la entidad aparece en GET /api/v1/trash filtrada por entity_type
```

### Escenario 2: Restore desde papelera

```gherkin
Dado que existe observation en papelera (deleted_at not null)
Cuando POST /api/v1/trash/restore con `{"entity_type":"observation","entity_id":"X"}`
Entonces `deleted_at = null`
Y se inserta audit_log "entity.restored"
Y vuelve a aparecer en queries normales
```

### Escenario 3: TTL purge

```gherkin
Dado que `DOMAIN_TRASH_TTL_DAYS=30`
Cuando un item tiene `deleted_at < now() - 30 días`
Y se ejecuta cron de purge diario
Entonces el item es hard-deleted (DELETE real)
Y attachments en S3 son eliminados
Y se logea audit_log "entity.purged"
```

### Escenario 4: Restore con conflicto

```gherkin
Dado que en papelera hay project con slug "acme"
Y existe ya project activo con slug "acme"
Cuando intento restore
Entonces error 409 "slug conflict"
Y se sugiere nuevo slug
```

### Escenario 5: Cascade soft-delete

```gherkin
Dado que borro un project con observaciones, prompts, knowledge_docs hijos
Cuando se procesa
Entonces todos los hijos quedan deleted_at = NOW() también
Y restore del project restaura todos los hijos también
```

### Escenario 6: Permisos

```gherkin
Dado que soy member (no admin)
Cuando intento ver papelera de otra org
Entonces 403
Y solo veo items que yo borré o de mis proyectos
```

## Análisis breve

- **Qué pide:** patrón soft-delete uniforme, vistas filtradas, papelera UI, purge cron, restore
- **Esfuerzo:** M
- **Riesgos:** todas las entidades necesitan migration; cascade complejo; queries existentes deben filtrar
