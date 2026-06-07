# Proposal: HU-13.2-obsidian-graph

## Intención

Generar el archivo `graph.json` que Obsidian consume para su Graph View. Cada observación activa es un nodo, cada relación en memory_relations es un link. Modos de configuración para controlar cuándo se regenera. Esto permite visualizar el grafo de conocimiento directamente desde Obsidian sin plugins adicionales.

## Scope

**Incluye:**

- Structs `Graph` (nodes + links), `GraphNode`, `GraphLink` en `internal/obsidian/graph.go`
- Función `GenerateGraph(ctx, reader, vaultPath, mode) error` que:
  - Lista todas las observaciones activas (no soft-deleted)
  - Lista todas las relaciones en memory_relations
  - Construye nodes[] y links[]
  - Escribe `{vault}/graph.json`
- Modos: `preserve` (no tocar si existe), `force` (regenerar siempre), `skip` (no hacer nada)
- Mapeo de relaciones: todas las relaciones de memory_relations se incluyen como links con su tipo original
- Nodos incluyen: id, type, title, project, tags (desde topic_key)
- Integración con CLI: `--graph-mode` flag en `engram obsidian export`

**No incluye:**

- Visualización personalizada (es formato estándar Obsidian)
- Filtros interactivos en el grafo
- Export de subgrafos por proyecto

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Formato | JSON estándar Obsidian Graph View: `{ nodes: [{id, type, title, project, tags}], links: [{source, target, relation, confidence}] }` |
| Archivo | `{vault}/graph.json` — raíz del vault |
| Modos | Enum `GraphMode: preserve | force | skip` |
| Check mode | Stat existente + switch |
| Nodos | Solo observaciones con `deleted_at IS NULL` |
| Links | Todas las filas de `memory_relations` donde source y target existen como nodos |

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| graph.json muy grande (>10MB) | Baja | --limit protege; si hay +10k obs, el grafo igual funciona en Obsidian |
| Relaciones huerfanas (target eliminado) | Baja | Se filtran: solo links donde source y target están en nodes |

## Testing

- **Unitario:** Graph building, mode logic, node/link filtering
- **Integración:** Generar graph con mock reader, verificar JSON output contra schema esperado
- **Modos:** Test para preserve, force, skip
