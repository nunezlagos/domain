# HU-39.1-clients-schema

**Origen:** `REQ-39-client-mandante-model`
**Prioridad tentativa:** alta (bloqueante para 39.3 / 39.4 / 39.5)
**Tipo:** schema / migration
**Wave:** 1 (sin dependencias)

## Historia de usuario

**Como** operador de la consultora (organización root)
**Quiero** una tabla `clients` per-organización que represente los mandantes
externos con los que trabajamos
**Para** que los proyectos puedan asociarse a un cliente concreto y el
dashboard pueda agruparlos por cuenta sin inventar abstracciones en jsonb
ni mezclarlos en `organizations`.

## Criterios de aceptación

### Escenario 1: La migración up corre limpia en una DB vacía

```gherkin
Dado un Postgres con migraciones 000001..000098 aplicadas
Cuando ejecuto `make migrate-up` (o el comando equivalente del runner)
Entonces la migración 000099_create_clients.up.sql aplica con exit 0 en <1s
Y la tabla `clients` existe con las columnas definidas en design.md
Y el constraint UNIQUE (organization_id, slug) está activo
Y el índice parcial clients_organization_id_idx existe WHERE deleted_at IS NULL
```

### Escenario 2: Aislamiento multi-tenant por organización

```gherkin
Dado dos organizations org_a y org_b en la misma DB
Cuando inserto en clients (organization_id=org_a, slug='acme-corp', name='Acme')
Y luego inserto en clients (organization_id=org_b, slug='acme-corp', name='Acme B')
Entonces ambos inserts son exitosos
Y un SELECT * FROM clients WHERE organization_id=org_a NO devuelve la fila de org_b
```

### Escenario 3: Slug duplicado dentro de la misma org falla

```gherkin
Dado un cliente existente con (org_a, 'acme-corp')
Cuando intento insertar otro cliente con (org_a, 'acme-corp')
Entonces el insert falla con SQLSTATE 23505 (unique_violation)
```

### Escenario 4: Soft delete preserva fila

```gherkin
Dado un cliente con id=$x y deleted_at=NULL
Cuando ejecuto UPDATE clients SET deleted_at=NOW() WHERE id=$x
Entonces la fila permanece en la tabla
Y SELECT con índice activo (WHERE deleted_at IS NULL) no la incluye
Y luego puedo crear OTRO cliente con el mismo slug y misma org sin colisión... NO
   (el UNIQUE constraint NO incluye deleted_at; reusar slug requiere hard delete o
   diferenciar con sufijo — limitación documentada)
```

### Escenario 5: ON DELETE CASCADE desde organizations

```gherkin
Dado una org con 3 clients
Cuando ejecuto DELETE FROM organizations WHERE id=$org
Entonces los 3 clients se eliminan en cascada
Y NO quedan filas huérfanas
```

### Escenario 6: Down migration es reversible

```gherkin
Dado la migración 000099 aplicada y datos de prueba en clients
Cuando ejecuto migrate down de 000099
Entonces la tabla clients desaparece sin error
Y las migraciones previas (000098) siguen intactas
```

## Notas

- Esta HU es **solo schema**. La lógica de aplicación (service / repo /
  handlers / mcp) se cubre en HU-39.3, 39.4, 39.5.
- RLS sobre `clients` se incluye en esta misma migración (no en REQ-40)
  porque el patrón se aplica desde la creación de la tabla, igual que se
  hizo con observations/sessions en migración 000085.
- La columna `metadata JSONB` queda como extensión futura (industria, sitio
  web, notas). No se modela schema fuerte porque varía por consultora.
