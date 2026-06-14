# HU-39.3-clients-service-and-repo

**Origen:** `REQ-39-client-mandante-model`
**Prioridad tentativa:** alta (bloqueante para 39.4, 39.5, 39.6)
**Tipo:** backend / domain layer
**Wave:** 2 (depende de 39.1 schema)

## Historia de usuario

**Como** developer backend que necesita exponer CRUD de clients por REST y
MCP
**Quiero** una capa `service.Client` con interfaz `Repository` + implementación
`pg_repository` que opere sobre la tabla `clients` respetando RLS y
el contexto multi-tenant
**Para** que handlers HTTP y tools MCP consuman la misma lógica de dominio
sin duplicar reglas (slug unique per-org, status válido, soft delete) y
sin manejar pgx directamente.

## Criterios de aceptación

### Escenario 1: Crear cliente con datos válidos

```gherkin
Dado un contexto con org_id=$org_a y user_id=$u
Cuando llamo svc.Create(ctx, CreateInput{Name:"Acme", Slug:"acme-corp"})
Entonces el método retorna un Client con id no vacío
Y el Client tiene organization_id=$org_a, status='active', deleted_at=nil
Y se ejecutó dentro de WithOrgTx($org_a)
Y la fila existe en DB
```

### Escenario 2: Crear cliente con slug duplicado per-org

```gherkin
Dado ya existe (org_a, 'acme-corp')
Cuando llamo svc.Create(ctx, {Name:"Acme 2", Slug:"acme-corp"})
Entonces retorna error tipado ErrSlugConflict
Y el handler/MCP tool puede mapearlo a 409 / código MCP equivalente
```

### Escenario 3: Listar clientes filtra por org actual

```gherkin
Dado clients en org_a y org_b
Y un contexto con org_id=$org_a
Cuando llamo svc.List(ctx, ListFilter{})
Entonces el resultado solo incluye clients de org_a
Y NO incluye clients con deleted_at != NULL (default filter)
```

### Escenario 4: Get por slug devuelve cliente activo

```gherkin
Dado un cliente activo (org_a, 'acme-corp')
Cuando llamo svc.GetBySlug(ctx, "acme-corp") con ctx de org_a
Entonces retorna el Client
Y si el cliente está soft-deleted o pertenece a otra org → ErrNotFound
```

### Escenario 5: Update preserva slug si no se cambia

```gherkin
Dado un cliente $c
Cuando llamo svc.Update(ctx, $c.ID, UpdateInput{Name:"Acme Inc."})
Entonces retorna el Client con Name actualizado
Y slug, organization_id, created_at no cambian
Y updated_at es > created_at
```

### Escenario 6: Archive es soft delete + status='archived'

```gherkin
Dado un cliente activo $c
Cuando llamo svc.Archive(ctx, $c.ID)
Entonces retorna nil error
Y la fila tiene status='archived' y deleted_at NOT NULL
Y svc.List(ctx, {}) no la incluye
Y svc.List(ctx, {IncludeArchived:true}) sí la incluye
```

### Escenario 7: Cross-tenant lookup falla

```gherkin
Dado un cliente $c en org_b
Cuando llamo svc.GetByID(ctx, $c.ID) con ctx de org_a
Entonces retorna ErrNotFound
Y la query SQL devolvió 0 rows (RLS + check explícito)
```

### Escenario 8: Validación de inputs

```gherkin
Dado un input con Slug inválido (mayúsculas, espacios, símbolos)
Cuando llamo svc.Create(ctx, {Slug:"Acme Corp!"})
Entonces retorna ErrInvalidInput con detalle de la regla violada
Y NO se intenta el INSERT (falla en validación previa)
```

## Notas

- Esta HU expone **interfaces y lógica de dominio**. No depende de
  routing HTTP ni de MCP. Esos consumen el service en HUs 39.4 y 39.5.
- El repo concreto se llama `pg_repository.go` siguiendo el patrón ya
  presente en `internal/service/project/`.
- Reutilizar `txctx.WithOrgTx` (existente desde REQ-25) para garantizar
  que cada operación corre dentro de tx con `SET LOCAL app.current_org_id`.
- Errores tipados: `ErrSlugConflict`, `ErrNotFound`, `ErrInvalidInput`.
  Mapping a HTTP/MCP queda fuera de esta HU.
