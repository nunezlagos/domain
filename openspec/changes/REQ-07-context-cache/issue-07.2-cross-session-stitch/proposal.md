# Proposal: issue-07.2-cross-session-stitch

## Intención

Crear un `SessionStitcher` que, al iniciar una nueva sesión, recupere resúmenes de sesiones anteriores, los fusione con dedup semántico, y genere un bloque de contexto continuo.

## Scope

**In scope:**
- Módulo `SessionStitcher` con método `Stitch(ctx, sessionID) -> StitchedContext`
- Recuperación de summaries de sesiones anteriores vía API de memoria (REQ-03)
- Dedup semántico basado en cosine similarity + exact key match (issue-03.6)
- Estructura de output: `Decisions`, `OpenItems`, `RecurringContext`, `SkippedSessions`
- Límite configurable de sesiones máximas a incluir

**Out of scope:**
- Stitching en tiempo real mientras la sesión actual corre
- Edición manual del stitched context
- Generación de summaries (eso es issue-03.2)

## Enfoque técnico

- `SessionStitcher` usa `MemoryStore.GetSessionSummaries()` para obtener summaries
- Cada summary es un objeto estructurado con `decisions[]`, `open_items[]`, `context_keys[]`
- Dedup engine compara por semantic key (hash de contenido normalizado) + cosine similarity > 0.92
- Items duplicados se colapsan y se agrega referencia cruzada de sesiones
- Output se formatea como texto plano estructurado para inyectar en system prompt

## Riesgos

- Dedup semántico puede ser caro (N embeddings queries por sesión) → limitar sesiones a stitch
- Summaries pueden ser muy grandes → aplicar truncamiento del 07.1
- Si no hay embeddings disponibles, dedup cae a exact match solamente

## Testing

- **Unit:** Stitcher con summaries mock, dedup engine, límite de sesiones
- **Integration:** Stitcher + MemoryStore real con sesiones seeded
- **Gherkin:** Escenarios del hu.md
- **Sabotaje:** Repetir misma decisión en 10 sesiones → verificar que aparece una vez con 10 referencias
