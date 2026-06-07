# HU-02.2-rbac

**Origen:** `REQ-02-auth-security`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** administrador de una organización
**Quiero** asignar roles (admin, developer, viewer) a los miembros y que el sistema valide permisos por entidad y acción
**Para** controlar quién puede crear, leer, actualizar, borrar o ejecutar recursos en mi organización

## Criterios de aceptación

### Escenario 1: Roles y permisos base

```gherkin
Dado que los roles están definidos como:
  | role      | permissions                                                       |
  | admin     | create, read, update, delete, execute sobre TODAS las entidades   |
  | developer | create, read, update, execute sobre agents/flows/skills/knowledge |
  | viewer    | read sobre todas las entidades                                    |
Cuando verifico la matriz de permisos
Entonces admin puede borrar cualquier entidad
Y developer NO puede borrar entidades
Y viewer solo puede leer
```

### Escenario 2: Admin puede crear/borrar recursos

```gherkin
Dado que soy un usuario con rol `admin`
Cuando intento crear un project
Entonces la operación es exitosa (200/201)

Cuando intento borrar un project existente
Entonces la operación es exitosa (200/204)
```

### Escenario 3: Developer NO puede borrar

```gherkin
Dado que soy un usuario con rol `developer`
Cuando intento crear un flow
Entonces la operación es exitosa

Cuando intento borrar un flow existente
Entonces recibo `403 Forbidden`
Y el body indica "insufficient permissions"
```

### Escenario 4: Viewer solo lectura

```gherkin
Dado que soy un usuario con rol `viewer`
Cuando intento leer un project
Entonces la operación es exitosa

Cuando intento crear un project
Entonces recibo `403 Forbidden`

Cuando intento actualizar un project
Entonces recibo `403 Forbidden`

Cuando intento borrar un project
Entonces recibo `403 Forbidden`

Cuando intento ejecutar un flow
Entonces recibo `403 Forbidden`
```

### Escenario 5: Organización scoping

```gherkin
Dado que existen dos organizaciones: `org-a` y `org-b`
Y soy admin de `org-a`
Cuando intento acceder a un recurso de `org-b`
Entonces recibo `404 Not Found`
Y NO recibo `403` (no debo revelar existencia del recurso)
```

### Escenario 6: Middleware verifica permisos por entidad

```gherkin
Dado que existe un endpoint `DELETE /api/v1/projects/:id`
Y existe un middleware de autorización
Cuando un usuario sin permiso de delete sobre projects intenta borrar
Entonces el middleware rechaza la petición antes de llegar al handler
```

## Análisis breve

- **Qué pide realmente:** Sistema RBAC con 3 roles, matriz de permisos por entidad y acción, middleware de autorización, scoping por organización (recursos aislados entre orgs).
- **Módulos sospechados:** `internal/auth/rbac/`, `internal/api/middleware/`, `internal/models/`
- **Riesgos / dependencias:** Scoping por organización es crítico — un error aquí expone datos de otras organizaciones. Los permisos deben verificarse en middleware a nivel de ruta.
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
