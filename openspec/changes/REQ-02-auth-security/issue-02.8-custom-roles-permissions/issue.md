# issue-02.8-custom-roles-permissions

**Origen:** `REQ-02-auth-security`
**Prioridad tentativa:** media
**Tipo:** feature

## Historia de usuario

**Como** owner de una organización Enterprise
**Quiero** definir roles custom con permisos fine-grained por recurso y acción
**Para** dar acceso parcial a auditores, contractors o equipos sin otorgar admin completo

## Modelo

- 5 roles built-in fijos (issue-02.2): owner, admin, maintainer, member, viewer
- **Custom roles** definibles por org: nombre + matriz de permisos `{resource:[actions]}`
- Resources: project, observation, session, prompt, knowledge_doc, skill, agent, flow, run, secret, member, plan, billing
- Actions: read, write, delete, execute, admin
- Resource-scoped: opcionalmente atar el permiso a IDs específicos (`project_ids:[X,Y]`)

## Criterios de aceptación

### Escenario 1: Crear rol custom

```gherkin
Dado que soy owner de org Acme
Cuando POST /api/v1/organizations/:id/roles con
  ```json
  {
    "slug": "auditor",
    "name": "Auditor",
    "permissions": {
      "project": ["read"],
      "observation": ["read"],
      "audit_log": ["read"]
    }
  }
  ```
Entonces se crea registro en `custom_roles`
Y la fila JSONB queda normalizada y validada
Y audit_log "role.created"
```

### Escenario 2: Asignar rol custom a member

```gherkin
Dado que existe rol `auditor` y user `bob`
Cuando POST /api/v1/organizations/:id/members/:user_id/role con `{"role":"auditor"}`
Entonces `users.role` queda como "auditor"
Y RBAC middleware resuelve permisos desde `custom_roles` (no built-in)
```

### Escenario 3: Resource-scoped permission

```gherkin
Dado que rol `contractor-projX` tiene `{"project":["read","write"], "scope":{"project_ids":["X"]}}`
Cuando bob (con ese rol) hace GET /projects/X
Entonces 200 OK
Cuando hace GET /projects/Y
Entonces 403 "no access to project Y"
```

### Escenario 4: Validación de permisos

```gherkin
Dado que envío permission `{"foo":["bar"]}` (resource inexistente)
Entonces 422 con error claro: "unknown resource 'foo'"
Y NO se persiste el rol
```

### Escenario 5: Role built-in no editable

```gherkin
Dado que intento PATCH /api/v1/organizations/:id/roles/admin
Entonces 403 "built-in roles are immutable"
```

### Escenario 6: Borrar rol con members asignados

```gherkin
Dado que rol custom tiene 3 members
Cuando DELETE /api/v1/organizations/:id/roles/:slug
Entonces 409 "role has 3 members assigned; reassign first"
```

## Análisis breve

- **Qué pide:** tabla custom_roles + matriz JSONB + RBAC engine que resuelve built-in OR custom
- **Esfuerzo:** M
- **Riesgos:** mal validation lleva a escalada; cache de permisos consistente al modificar
