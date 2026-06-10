# Design: issue-12.2-mcp-memory-tools

## Decisión arquitectónica

```
                    ┌─────────────────────────────┐
                    │     MCPServer (issue-12.1)     │
                    │                             │
                    │  RegisterTool("domain_mem_save",   │
                    │    memSaveHandler)          │
                    │  RegisterTool("domain_mem_search", │
                    │    memSearchHandler)        │
                    │  ... (12 tools)             │
                    └──────────┬──────────────────┘
                               │
                    ┌──────────┴──────────────────┐
                    │    MemoryService             │
                    │    (internal/service/)       │
                    │                              │
                    │  Save(ctx, req) → Observation│
                    │  Search(ctx, req) → []Result │
                    │  Get(ctx, id) → Observation  │
                    │  Delete(ctx, id) → error     │
                    │  Timeline(ctx, req) → []Obs  │
                    │  Context(ctx, req) → Context │
                    │  Stats(ctx, project) → Stats │
                    │  SessionStart/End/Summary    │
                    │  SavePrompt(ctx, req)        │
                    │  CapturePassive(ctx, req)    │
                    │  SuggestTopicKey(ctx, req)   │
                    └──────────┬──────────────────┘
                               │
                    ┌──────────┴──────────────────┐
                    │    DB Layer (internal/db/)   │
                    │                              │
                    │  observations TABLE          │
                    │  sessions TABLE              │
                    │  prompts TABLE               │
                    │  pgvector for embeddings     │
                    │  content_tsv for full-text   │
                    └──────────────────────────────┘
```

**Decisión:** Separación clara en 3 capas: MCP handlers (solo routing y validación) → Service (lógica de negocio) → DB (acceso a datos). Los handlers MCP son thin adapters que convierten requests MCP a llamadas de servicio y viceversa.

## Diagrama de datos

```
observations TABLE:
  id              UUID PK
  project_id      UUID FK → projects
  session_id      UUID nullable
  title           VARCHAR(500)
  content         TEXT
  content_tsv     TSVECTOR (GENERATED)
  embedding       VECTOR(1536) nullable
  type            VARCHAR(50)     -- decision, fix, pattern, context, artifact, session, prompt, passive
  scope           VARCHAR(50)     -- project, personal
  topic_key       VARCHAR(200) nullable
  source          VARCHAR(200) nullable  -- para capture_passive
  metadata        JSONB nullable
  deleted_at      TIMESTAMPTZ nullable   -- soft delete
  created_at      TIMESTAMPTZ
  updated_at      TIMESTAMPTZ

sessions TABLE:
  id              VARCHAR(100) PK
  project_id      UUID FK → projects
  directory       TEXT
  status          VARCHAR(20)     -- active, completed
  summary         TEXT nullable
  started_at      TIMESTAMPTZ
  ended_at        TIMESTAMPTZ nullable
```

## TDD plan

1. **Red:** Test que `domain_mem_save` guarda observation en DB y devuelve id
2. **Green:** Implementar handler + MemoryService.Save()
3. **Red:** Test que `domain_mem_search` busca por query y devuelve resultados
4. **Green:** Implementar MemoryService.Search() con pgvector
5. **Red:** Test que `domain_mem_get_observation` devuelve observation por id
6. **Green:** Implementar MemoryService.Get()
7. **Red:** Test que `domain_mem_delete` marca deleted_at
8. **Green:** Implementar soft delete
9. **Red:** Test que `domain_mem_timeline` devuelve observaciones alrededor de un id
10. **Green:** Implementar Timeline() con query paginada por created_at
11. **Red:** Test que `domain_mem_session_start` crea sesión activa
12. **Green:** Implementar SessionStart/End/Summary
13. **Red:** Test que `domain_mem_stats` devuelve counts agrupados
14. **Green:** Implementar Stats() query agregada
15. **Sabotaje:** domain_mem_delete sin soft delete → datos se pierden

## Riesgos y mitigación

- **Embedding service dependency:** Si falla, domain_mem_search se degrada a full-text. Mitigación: fallback automático, log warning.
- **Large result sets:** domain_mem_search sin límite puede devolver miles de resultados. Mitigación: limit default 10, max 100.
- **Session management:** Sesiones abiertas sin cerrar. Mitigación: TTL de sesiones (24h), auto-close on timeout.
- **Input validation:** Argumentos inválidos pueden causar SQL injection. Mitigación: parametrized queries siempre, validación estricta en handler.
