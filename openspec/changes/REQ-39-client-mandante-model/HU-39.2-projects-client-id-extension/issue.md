# HU-39.2-projects-client-id-extension

**Origen:** `REQ-39-client-mandante-model`
**Prioridad tentativa:** alta (bloqueante para 39.6)
**Tipo:** schema / migration
**Wave:** 1 (sin dependencias con código Go; corre paralelo a HU-39.1)

## Historia de usuario

**Como** operador de la consultora
**Quiero** que cada `project` pueda apuntar a un `client` opcionalmente
**Para** que la consultora distinga entre proyectos internos (sin cliente)
y proyectos contratados por un mandante (con `client_id` poblado), sin
perder histórico cuando un cliente se elimina.

## Criterios de aceptación

### Escenario 1: Migración up agrega columna sin romper datos existentes

```gherkin
Dado un Postgres con migraciones 000001..000099 y N proyectos sin cliente
Cuando ejecuto `make migrate-up` para aplicar 000100
Entonces todas las filas existentes de projects tienen client_id = NULL
Y la columna client_id es nullable
Y existe FK projects.client_id → clients.id con ON DELETE SET NULL
Y existe índice parcial projects_client_id_idx WHERE deleted_at IS NULL
   AND client_id IS NOT NULL
```

### Escenario 2: Insertar proyecto sin cliente (compatibilidad legacy)

```gherkin
Dado la migración 000100 aplicada
Cuando inserto en projects (organization_id, slug, name) sin pasar client_id
Entonces el insert es exitoso
Y client_id queda NULL (default)
```

### Escenario 3: Insertar proyecto con cliente válido

```gherkin
Dado un cliente $c en org_a y la sesión con SET LOCAL app.current_org_id=org_a
Cuando inserto un proyecto con (organization_id=org_a, client_id=$c, slug=x)
Entonces el insert es exitoso
Y el trigger same-org no se dispara con error
```

### Escenario 4: Cross-org rechazado por trigger

```gherkin
Dado un cliente $c en org_b
Cuando intento insertar projects (organization_id=org_a, client_id=$c, ...)
Entonces el insert falla con error 'check_violation'
Y el mensaje indica que client.organization_id no coincide con
   project.organization_id
```

### Escenario 5: Borrar cliente deja proyectos huérfanos (NO cascade)

```gherkin
Dado un cliente $c con 3 proyectos asociados
Cuando ejecuto DELETE FROM clients WHERE id=$c
Entonces los 3 proyectos persisten en la tabla
Y todos tienen client_id = NULL (SET NULL del FK)
```

### Escenario 6: UPDATE de client_id revalida same-org

```gherkin
Dado un proyecto con organization_id=org_a y client_id=NULL
Y un cliente $c en org_b
Cuando ejecuto UPDATE projects SET client_id=$c WHERE id=$proj
Entonces el UPDATE falla con check_violation
```

### Escenario 7: Down migration es reversible

```gherkin
Dado la migración 000100 aplicada con datos en projects.client_id
Cuando ejecuto migrate down de 000100
Entonces la columna client_id desaparece de projects
Y la función projects_check_client_same_org() desaparece
Y la tabla projects sigue intacta (filas, columnas previas, otros índices)
```

## Notas

- Esta HU NO toca código Go ni SDKs. La integración del campo en el
  service/repo/handler/mcp ocurre en HU-39.6.
- El trigger es necesario porque Postgres no permite subqueries en
  `CHECK` constraints; la única alternativa correcta sin trigger sería
  desnormalizar (replicar organization_id en clients _y_ usar CHECK
  composite), descartado por mayor complejidad.
- `ON DELETE SET NULL` es decisión deliberada: una consultora no debe
  perder histórico de trabajo si por error borra un cliente.
