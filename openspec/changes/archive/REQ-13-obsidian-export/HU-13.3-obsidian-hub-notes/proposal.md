# Proposal: HU-13.3-obsidian-hub-notes

## Intención

Generar notas índice (hub notes) que agrupan observaciones por sesión y por tópico, usando el formato de wikilinks de Obsidian. Las session hubs permiten navegar "qué pasó en esa sesión". Las topic hubs permiten descubrir clusters de conocimiento reuniendo todas las observaciones que comparten un topic_key.

## Scope

**Incluye:**

- `GenerateSessionHubs(ctx, reader, vaultPath, slugMap) error` — por cada sesión con observaciones, crea `_sessions/{id}.md`
- `GenerateTopicHubs(ctx, reader, vaultPath, slugMap) error` — por cada topic_key con >= 2 observaciones, crea `_topics/{prefix}.md`
- Frontmatter específico para cada tipo de hub (session-hub, topic-hub)
- Body con metadata + lista de wikilinks
- Agrupación por type en topic hubs
- Integración al final del Export pipeline
- Regeneración completa en cada export (siempre fresh)

**No incluye:**

- Navegación cross-hub (ej: de session a topic y viceversa)
- Ordenamiento personalizado de wikilinks
- Filtros en hub notes

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Archivos | `_sessions/{session_id}.md` y `_topics/{topic_prefix}.md` |
| Prefijo `_` | Convención Obsidian para archivos "meta" que aparecen primero en el explorador |
| Frontmatter | type específico + metadata relevante |
| Body | Título markdown + metadata + lista wikilinks |
| Regeneración | Siempre fresh (se sobreescribe en cada export) |
| SlugMap | Recibe el mismo slugMap de HU-13.1 para consistencia de wikilinks |
| Topics threshold | Solo si count >= 2 observaciones |

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| Muchas sesiones → muchos archivos _sessions/ | Media | Cada sesión es un archivo pequeño; Obsidian maneja miles sin problema |
| Topic_key puede ser largo para filename | Baja | Se usa el prefix tal cual; si tiene caracteres especiales, se slugifica |

## Testing

- **Unitario:** Session hub rendering, topic hub rendering, grouping logic
- **Integración:** Export con mock reader + verificar archivos generados
- **Threshold:** Topic con 1 obs no genera archivo; con 2+ sí
