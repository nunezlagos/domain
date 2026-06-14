# HU-39.5-clients-mcp-tools

**Origen:** `REQ-39-client-mandante-model`
**Prioridad tentativa:** alta
**Tipo:** backend / MCP server
**Wave:** 3 (depende de 39.3, paralelo a 39.4)

## Historia de usuario

**Como** consumidor del MCP server (Claude Code, agente automatizado)
**Quiero** tools MCP `client.create`, `client.list`, `client.get`,
`client.update`, `client.archive`, `client.restore`
**Para** poder operar clientes desde agentes con la misma semántica que
los endpoints REST, sin duplicar lógica.

## Criterios de aceptación

### Escenario 1: client.create vía POST /mcp

```gherkin
Dado un Bearer token MCP válido para org_a
Cuando envío JSON-RPC con method="tools/call" name="client.create"
   args={"name":"Acme","slug":"acme-corp"}
Entonces la respuesta es 200 con result.content describiendo el cliente
   creado (id, slug, status)
Y la fila existe en DB bajo org_a
```

### Escenario 2: client.list aislada por org

```gherkin
Dado clientes en org_a y org_b
Cuando llamo tools/call client.list desde Bearer de org_a
Entonces el resultado solo contiene clients de org_a
Y NO aparece ninguno de org_b
```

### Escenario 3: client.get por slug

```gherkin
Dado un cliente activo
Cuando llamo client.get con {"slug":"acme-corp"}
Entonces retorna el cliente completo
Y si el slug pertenece a otra org → tool error tipado not_found
```

### Escenario 4: client.update parcial

```gherkin
Dado un cliente $c
Cuando llamo client.update con {"slug":"acme-corp","name":"Acme Inc."}
Entonces retorna el cliente actualizado
Y solo Name cambia
```

### Escenario 5: client.archive

```gherkin
Dado un cliente activo
Cuando llamo client.archive con {"slug":"acme-corp"}
Entonces retorna { ok: true }
Y client.get con el mismo slug → not_found
Y client.list con include_archived=true → sí lo muestra
```

### Escenario 6: client.restore

```gherkin
Dado un cliente archivado
Cuando llamo client.restore
Entonces retorna el cliente con status='active', deleted_at=null
Y queda visible en client.list default
```

### Escenario 7: Tool descubrible

```gherkin
Dado un cliente MCP que invoca tools/list
Cuando recibe el catalog
Entonces aparecen client.create, client.list, client.get, client.update,
   client.archive, client.restore
Y cada tool incluye inputSchema JSON Schema válido
```

### Escenario 8: Validación de inputs

```gherkin
Dado un agente que llama client.create con args={"name":"X"}
   (falta slug)
Cuando el tool se ejecuta
Entonces retorna error tipado invalid_input con detalle "slug required"
Y NO se llama al service
```

## Notas

- Cada tool delega al mismo `service.Client` del HU-39.3. No duplicar
  lógica.
- Schema input/output JSON Schema para auto-validación y descubrimiento.
- Patrón ya consolidado en `internal/mcp/server/project_tools.go` —
  imitarlo.
- Bearer MCP ya valida org en middleware (REQ-31). El handler tool solo
  extrae orgID del context.
