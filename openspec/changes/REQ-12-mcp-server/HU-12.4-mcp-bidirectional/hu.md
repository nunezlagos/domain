# HU-12.4-mcp-bidirectional

**Origen:** `REQ-12-mcp-server`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** plataforma Domain
**Quiero** consumir servidores MCP externos y exponerlos como skills internos, además de proveer nuestros propios tools
**Para** que los agentes de Domain puedan usar herramientas de terceros (bases de conocimiento, APIs externas, etc.) sin configuración manual

## Criterios de aceptación

### Escenario 1: Registrar servidor MCP externo

```gherkin
Dado un servidor MCP externo accesible (ej: `npx @modelcontextprotocol/server-github`)
Cuando registro el servidor en Domain con:
  | name      | "github-mcp"        |
  | transport | "stdio"             |
  | command   | "npx"               |
  | args      | ["@modelcontextprotocol/server-github"] |
  | env       | {"GITHUB_TOKEN":"..."} |
Entonces Domain conecta con el servidor MCP externo
Y descubre automáticamente todas sus tools
Y las tools se almacenan en el MCP Hub registry
```

### Escenario 2: Tools externas se convierten en skills

```gherkin
Dado que el servidor MCP "github-mcp" está registrado y conectado
Cuando se descubren sus tools (ej: `github_list_issues`, `github_create_issue`)
Entonces se crea automáticamente un skill por cada tool
Y cada skill tiene:
  | name        | "github_list_issues"     |
  | source      | "mcp:github-mcp"         |
  | description | (tomada del tool MCP)    |
  | inputSchema | (tomada del tool MCP)    |
Y el skill queda disponible para ejecutarse como cualquier skill nativo
```

### Escenario 3: Ejecutar tool externa vía skill

```gherkin
Dado que el skill `github_list_issues` está creado a partir del MCP externo
Cuando ejecuto el skill (vía domain_skill_execute MCP o flow step)
Entonces Domain traduce la ejecución a una llamada `tools/call` al servidor MCP externo
Y el servidor externo ejecuta la tool
Y el resultado se devuelve como si fuera un skill nativo
```

### Escenario 4: Tools de Domain disponibles para MCP externos

```gherkin
Dado un servidor MCP externo que quiere consumir tools de Domain
Cuando el servidor MCP de Domain está activo (HU-12.1)
Entonces el cliente MCP externo puede listar todas las tools de Domain
Y puede invocar tools de memoria, agentes, flows, etc.
```

### Escenario 5: Heartbeat y reconexión con MCP externo

```gherkin
Dado un servidor MCP externo conectado por stdio
Cuando el proceso externo termina inesperadamente
Entonces Domain detecta la desconexión (stdin EOF)
Y marca el servidor como "disconnected"
Y reintenta la conexión cada 30 segundos (hasta 5 intentos)
Y después del límite, marca como "failed"
```

### Escenario 6: Tools discovery periódico

```gherkin
Dado un servidor MCP externo conectado
Cuando pasa el intervalo de discovery (cada 5 minutos)
Entonces Domain envía `tools/list` al servidor externo
Y actualiza el registro de tools si hay cambios
Y crea/actualiza/elimina skills según corresponda
```

### Escenario 7: Error en tool externa se propaga correctamente

```gherkin
Dado un servidor MCP externo registrado
Cuando invoco un skill basado en tool externa y el servidor devuelve error
Entonces el error MCP se traduce a un error de skill
Y el mensaje de error incluye el error original del servidor externo
Y el skill_run se marca como `failed`
```

## Análisis breve

- **Qué pide realmente:** Sistema bidireccional MCP donde Domain es servidor (provee tools) y cliente (consume MCPs externos). Los MCPs externos se convierten automáticamente en skills. Incluye hub registry, discovery automático y ciclo de vida de conexiones.
- **Módulos sospechados:** `internal/mcp/hub/`, `internal/mcp/client/`, `internal/mcp/skill-adapter/`, `internal/service/skill/mcp.go`
- **Riesgos / dependencias:** Depende de HU-12.1 (core MCP server). Procesos externos (stdio) pueden ser inestables. Seguridad al ejecutar comandos arbitrarios.
- **Esfuerzo tentativo:** XL

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
