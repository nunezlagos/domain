# HU-39.6-projects-accept-client-slug

**Origen:** `REQ-39-client-mandante-model`
**Prioridad tentativa:** alta (cierra el ciclo del REQ)
**Tipo:** backend / service + handler + MCP
**Wave:** 4 (depende de 39.2 schema + 39.3 client service)

## Historia de usuario

**Como** operador de la consultora creando proyectos para un mandante
**Quiero** que `POST /api/v1/projects` y `tools/call project.create`
acepten `client_slug` opcional
**Para** que el proyecto quede asociado al cliente correcto sin tener
que conocer su UUID y para que el dashboard pueda filtrar/agrupar
proyectos por cliente.

## Criterios de aceptación

### Escenario 1: Crear proyecto sin cliente (legacy preservado)

```gherkin
Dado un POST /api/v1/projects con body
   {"name":"X","slug":"x"}
Cuando el handler procesa
Entonces 201 Created
Y la fila tiene client_id = NULL
Y el response NO incluye `client_slug` (o lo incluye como null)
```

### Escenario 2: Crear proyecto con client_slug válido

```gherkin
Dado un cliente activo "acme-corp" en org_a
Cuando hago POST /api/v1/projects con
   {"name":"X","slug":"x","client_slug":"acme-corp"} como user de org_a
Entonces 201 Created
Y projects.client_id apunta al UUID del cliente acme-corp
Y el response incluye `client_slug:"acme-corp"`
```

### Escenario 3: client_slug inexistente → 422

```gherkin
Dado que NO existe cliente "ghost" en org_a
Cuando hago POST /api/v1/projects con client_slug="ghost"
Entonces 422 Unprocessable Entity
Y body: { "error":"client_not_found","message":"client 'ghost' not found in
   organization" }
Y NO se insertó nada en projects
```

### Escenario 4: client_slug de otra org → 422 (no 404)

```gherkin
Dado existe "acme-corp" en org_b
Cuando hago POST /api/v1/projects con client_slug="acme-corp" como org_a
Entonces 422 (porque desde el ctx org_a no se ve)
Y NO se filtra información cross-org (404 vs 422 indistinguible en respuesta)
```

### Escenario 5: client_slug de cliente archivado → 422

```gherkin
Dado un cliente "old-corp" en org_a con status='archived'
Cuando hago POST /api/v1/projects con client_slug="old-corp"
Entonces 422 con error "client_archived"
Y no se permite asociar a clientes archivados
```

### Escenario 6: GET /api/v1/projects expone client info

```gherkin
Dado un proyecto con client_id apuntando a "acme-corp"
Cuando hago GET /api/v1/projects/{slug}
Entonces el response incluye `client_slug:"acme-corp"` y
   `client_name:"Acme"`
Y un proyecto sin cliente expone `client_slug:null`
```

### Escenario 7: GET /api/v1/projects?client_slug=acme-corp filtra

```gherkin
Dado 5 proyectos en org_a, 2 de ellos con client_slug='acme-corp'
Cuando hago GET /api/v1/projects?client_slug=acme-corp
Entonces 200 OK
Y items.length == 2
Y todos tienen client_slug == "acme-corp"
```

### Escenario 8: MCP project.create también acepta client_slug

```gherkin
Dado tools/call project.create con args
   {"name":"X","slug":"x","client_slug":"acme-corp"} bajo Bearer org_a
Entonces el cliente queda asociado
Y la respuesta incluye client_slug
```

### Escenario 9: PATCH /projects/{slug} permite cambiar client

```gherkin
Dado un proyecto con client_id=NULL
Cuando hago PATCH con {"client_slug":"acme-corp"}
Entonces 200 OK
Y projects.client_id ahora apunta al UUID correcto
Cuando hago PATCH con {"client_slug":null}
Entonces 200 OK
Y projects.client_id vuelve a NULL (desasocia)
```

## Notas

- El handler/MCP solo conoce `client_slug` (strings). El service de
  projects resuelve internamente slug → UUID consultando `service.Client`.
- La resolución corre dentro del mismo WithOrgTx que el INSERT/UPDATE
  para evitar TOCTOU (cliente borrado entre lookup y insert).
- Si `client_slug` está presente en el JSON como `null` explícito, se
  interpreta como "desasociar" (PATCH). Si está ausente, se ignora.
- No cambia el comportamiento del trigger DB; es defense-in-depth.
