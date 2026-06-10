# Design: issue-04.6-mcp-profiles-resolution

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativas |
|----------|---------------|--------------|
| Perfiles | Mapa estático al start | Per-dynamic request profile |
| Middleware | Wrapper funcional alrededor de ToolHandler | Decorator pattern, interceptor |
| Envelope | Struct genérico con result type parameter | Mapa dinámico (pierde type safety) |
| Project cache | Cache en memoria por proceso | Resolver en cada request (lento) |

Se elige perfil estático porque la tool list se negocia en el handshake `initialize` y no puede cambiar luego. El middleware funcional es el patrón Go más idiomático para cross-cutting concerns. El envelope con type parameter (Go 1.18+ generics) permite type safety sin casting.

## Alternativas descartadas

- **Perfiles dinámicos:** El protocolo MCP no soporta renegociación de capabilities. Debe ser fijo al start.
- **Interceptor pesado estilo Java:** El middleware funcional es más liviano y testable.
- **Mapa `map[string]any` como envelope:** Requiere type assertion en cada handler. Generics dan safety.

## Diagrama

```
Server Startup:
  ┌─────────────┐
  │ Parse flags  │──► profile := flag.String("profile", "default")
  └──────┬──────┘
         ▼
  ┌─────────────┐
  │ Get profile  │──► toolNames := profiles[profile]
  └──────┬──────┘      │
         ▼              ▼
  ┌──────────────────────────────┐
  │ Filter tool registry         │
  │ Keep only toolNames in map   │
  └──────────────────────────────┘

Per-Request Resolution:
  ┌─────────────────────┐
  │ Incoming Tool Call  │
  └────────┬────────────┘
           ▼
  ┌──────────────────────────────────┐
  │ Project Resolution Middleware    │
  │                                  │
  │  1. Request has "project"?       │
  │     ├──► Yes → use it            │
  │     └──► No → next              │
  │                                  │
  │  2. "all_projects" = true?       │
  │     ├──► Yes → bypass project    │
  │     └──► No → next              │
  │                                  │
  │  3. Resolve from cwd:            │
  │     ├──► ENGRAM_PROJECT? → env   │
  │     ├──► Config file? → config   │
  │     ├──► Git remote? → git_remote│
  │     ├──► Git root? → git_root    │
  │     ├──► Git child? → ambiguous  │
  │     └──► Dir basename → fallback │
  │                                  │
  │  4. Wrap result in envelope:     │
  │     { project, project_source,   │
  │       project_path, result }     │
  └──────────────────────────────────┘
           ▼
  ┌─────────────────────┐
  │ Tool Handler        │
  │ (receives resolved   │
  │  project in ctx)     │
  └─────────────────────┘
```

## TDD plan

**Red:**
1. `TestDefaultProfile`: perfil default → 19 tools registradas
2. `TestAgentProfile`: perfil agent → 14 tools, admin ausentes
3. `TestInvalidProfile`: error al start
4. `TestProjectMiddlewareExplicit`: request con project → se pasa igual
5. `TestProjectMiddlewareImplicit`: request sin project → resuelve de cwd
6. `TestProjectMiddlewareAllProjects`: all_projects=true → bypass
7. `TestEnvelopeStructure`: response tiene project, project_source, project_path, result
8. `TestENGRAM_PROJECTOverride`: env set → todas las tools usan ese project
9. `TestProjectCache`: segunda request usa cache, no re-resuelve

**Green:** Profile map, middleware wrapper, envelope struct, cache.

**Refactor:** Extraer middleware a `internal/mcp/middleware.go`.

**Sabotaje:** Profile name con typo → server no arranca. Middleware sin project en request y cwd sin git → dir_basename funcional.

## Riesgos y mitigación

- **Cache de proyecto stale:** El cwd no cambia durante la vida del proceso MCP. Si cambia, el cliente debe reiniciar el server. Documentado.
- **ENGRAM_PROJECT vs project explícito:** El orden es: (1) parámetro `project` en request, (2) `all_projects`, (3) `ENGRAM_PROJECT`, (4) resolución desde cwd. El parámetro explícito gana siempre.
- **Profile flag inconsistente con env:** `MEM_MCP_PROFILE` env y `--profile` flag. Flag gana si ambos están presentes (conventional).
