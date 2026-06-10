# issue-02.4-audit-log

**Origen:** `REQ-02-auth-security`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** administrador de seguridad
**Quiero** un registro de auditoría inmutable que capture cada acción importante (actor, acción, entidad, valores anterior/nuevo, timestamp, IP)
**Para** poder investigar incidentes de seguridad, auditar cambios y cumplir con requisitos de compliance

## Criterios de aceptación

### Escenario 1: Registro de una acción

```gherkin
Dado que existe un usuario autenticado con id `user-123`
Cuando el usuario crea un project con nombre "mi-proyecto"
Entonces se registra un audit log con:
  | actor_id      | user-123                 |
  | action        | create                   |
  | entity_type   | project                  |
  | entity_id     | el UUID del project      |
  | new_values    | {"name":"mi-proyecto"}  |
  | old_values    | null                     |
  | ip_address    | la IP del request        |
  | occurred_at   | timestamp actual          |
```

### Escenario 2: Auditoría de actualización con valores anterior/nuevo

```gherkin
Dado que existe un project con nombre "viejo-nombre"
Cuando un usuario cambia el nombre a "nuevo-nombre"
Entonces se registra un audit log con:
  | action      | update          |
  | old_values  | {"name":"viejo-nombre"} |
  | new_values  | {"name":"nuevo-nombre"} |
```

### Escenario 3: Auditoría de eliminación

```gherkin
Dado que existe un project
Cuando un usuario lo elimina
Entonces se registra un audit log con:
  | action      | delete                     |
  | old_values  | {"name":"nombre-del-project", "id":"uuid", ...} |
  | new_values  | null                       |
```

### Escenario 4: El log es immutabile (append-only)

```gherkin
Dado que existen registros de auditoría
Cuando intento UPDATE o DELETE sobre la tabla audit_log
Entonces la operación falla (no hay permisos o triggers lo impiden)
Y los registros existentes no se modifican
```

### Escenario 5: Consulta de auditoría por actor

```gherkin
Dado que existen registros de auditoría para diferentes usuarios
Cuando consulto los audit logs filtrando por actor_id = "user-123"
Entonces recibo solo los registros donde actor_id es "user-123"
Y los resultados están ordenados por occurred_at DESC
```

### Escenario 6: Consulta por entidad y acción

```gherkin
Dado que existen registros de auditoría de varios tipos
Cuando consulto filtrando por entity_type = "project" AND action = "delete"
Entonces recibo solo las eliminaciones de projects
```

### Escenario 7: Política de retención (90 días)

```gherkin
Dado que existen audit logs con más de 90 días de antigüedad
Cuando ejecuto el comando `domain audit prune`
Entonces los logs más antiguos que 90 días se eliminan
Y los logs de menos de 90 días permanecen
```

## Análisis breve

- **Qué pide realmente:** Tabla append-only para auditoría inmmutable con actor, acción, entidad, valores anterior/nuevo, IP, timestamp. Consultas por actor/entidad/acción. Política de retención configurable.
- **Módulos sospechados:** `internal/audit/`, `internal/api/middleware/audit.go`
- **Riesgos / dependencias:** La inmutabilidad debe enforced desde la app (no hacer UPDATE/DELETE) y desde la DB (REVOKE privileges o trigger). La tabla puede crecer mucho.
- **Esfuerzo tentativo:** M

## Verificación previa

- [ ] Revisar codebase (grep)
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** pendiente
- **Evidencia:**
- **Acción derivada:**
