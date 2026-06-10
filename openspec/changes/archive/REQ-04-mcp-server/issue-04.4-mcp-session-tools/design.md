# Design: issue-04.4-mcp-session-tools

## Decisión arquitectónica

| Decisión | Opción elegida | Alternativas |
|----------|---------------|--------------|
| Store de sesiones | Tabla separada `sessions` en SQLite | Embed en tabla de observaciones |
| Session ID | UUID v4 si no se provee | Siempre requerido del cliente |
| Idempotencia | `SELECT` antes de `INSERT` | `INSERT OR IGNORE` con ON CONFLICT DO NOTHING |
| Markdown parsing | Regex section-based | Parser AST completo (goldmark) |

Se usa tabla separada `sessions` para mantener el modelo de dominio limpio y permitir queries eficientes (sesiones activas, duración promedio, etc.). UUID v4 como fallback para IDs opcionales. Regex para markdown porque el AST completo es overkill para 3 secciones predecibles.

## Alternativas descartadas

- **Goldmark AST parser:** Dependencia pesada (~2MB) para parsear 3 secciones de markdown. Regex es suficiente y más rápido.
- **Session ID obligatorio del cliente:** Forzar al cliente a generar IDs es mala UX. UUID v4 fallback automático.

## Diagrama

```
domain_mem_session_start(request)
  │
  ├─► Validate: id required (auto-generate UUID if empty)
  ├─► Check if session exists
  │     ├─► Exists → return existing (idempotent)
  │     └─► Not exists → INSERT new session
  └─► Return { id, directory, status: "active", started_at }

domain_mem_session_end(request)
  │
  ├─► Validate: id required
  ├─► Find session by id
  │     ├─► Not found → error "session not found"
  │     ├─► Status=ended → error "session already ended"
  │     └─► Active → UPDATE status=ended, ended_at=now
  │         └─► If summary provided → save as observation
  └─► Return { id, status: "ended", ended_at, duration_seconds }

domain_mem_session_summary(request)
  │
  ├─► Validate: id + content required
  ├─► Find session → must exist (may be active or ended)
  ├─► Parse markdown sections:
  │     ├─► "## Accomplished" → items → type="context"
  │     ├─► "## Decisions" → items → type="decision"
  │     └─► "## Next Steps" → items → type="context"
  ├─► Save each item as separate observation (linked to session)
  ├─► Save raw summary as observation type="session"
  ├─► Mark session has_summary=true
  └─► Return { id, items_created: N, has_summary: true }

domain_mem_capture_passive(request)
  │
  ├─► Validate: content required (min 3 chars)
  ├─► Determine type: heuristic
  │     ├─► Contains "discovered","found","realized" → "pattern"
  │     └─► Else → "context"
  ├─► Create observation with content
  ├─► Link to session_id if provided
  └─► Return { id, type, session_id }
```

## TDD plan

**Red:**
1. `TestSessionStart`: crear sesión, verificar campos
2. `TestSessionStartIdempotent`: mismo id dos veces → misma sesión
3. `TestSessionEnd`: cerrar sesión activa → status=ended
4. `TestSessionEndNotFound`: id inexistente → error
5. `TestSessionEndAlreadyEnded`: doble end → error
6. `TestSessionEndWithSummary`: summary en end → se guarda
7. `TestSessionSummary`: markdown parseado → N observaciones creadas
8. `TestSessionSummaryNoSections`: markdown sin secciones → raw guardado
9. `TestCapturePassive`: texto → observation type pattern/context
10. `TestCapturePassiveWithSession`: vinculado a sesión
11. `TestCapturePassiveEmptyContent`: error

**Green:** Implementar session store, handlers, markdown parser simplificado.

**Refactor:** Extraer markdown parser, validators.

**Sabotaje:** session_end sin session_start → error confirmado. markdown con `# Accomplished` (single `#`) → no debe parsear (solo `##`).

## Riesgos y mitigación

- **UUID dependency:** `github.com/google/uuid` — liviana, estándar en Go ecosystem.
- **Concurrencia en sesiones:** Una sesión por proceso normalmente. Si hay concurrencia, usar `sync.Mutex` en session store.
