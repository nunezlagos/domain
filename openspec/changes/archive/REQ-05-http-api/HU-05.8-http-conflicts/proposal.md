# Proposal: HU-05.8-http-conflicts

## Intención

Exponer 8 endpoints para el ciclo completo de detección y resolución de conflictos: listar grupos de conflictos, juzgar semánticamente (usando modelo externo), comparar dos observaciones, obtener detalle, estadísticas, escaneo lexical/semántico, listar deferred (apply diferido) y replay.

## Scope

**Incluye:**
- `GET /conflicts` — listar grupos de conflictos (observaciones con normalized_hash duplicado)
- `POST /conflicts/judge` — juzgar conflicto semánticamente vía modelo externo
- `POST /conflicts/compare` — comparar dos observaciones, retornar similarity score y diferencias
- `GET /conflicts/{id}` — obtener detalle de un conflicto
- `GET /conflicts/stats` — estadísticas de conflictos (total, resolved, pending, by_project)
- `POST /conflicts/scan` — escanear DB en busca de nuevos conflictos (lexical + opcional semántico)
- `GET /conflicts/deferred` — listar apply diferidos
- `POST /conflicts/deferred/replay` — replay de deferred

**No incluye:**
- Autenticación (HU-05.9)
- Resolución automática de conflictos
- Interfaz de usuario para conflictos

## Enfoque técnico

| Aspecto | Decisión |
|---------|----------|
| Conflict detection | Delegar a `conflict.LexicalScan(db)` y `conflict.SemanticJudge(db, model)` |
| Judge | POST /conflicts/judge acepta array de observation_ids + model name; delega en `conflict.Judge` |
| Compare | Comparación pairwise vía normalized_hash y/o Jaccard similarity |
| Stats | Queries agregadas sobre tabla memory_relations con judgment_status |
| Deferred | Query sobre sync_apply_deferred + replay via `conflict.ReplayDeferred` |

## Riesgos

| Riesgo | Probabilidad | Mitigación |
|--------|-------------|------------|
| Dependencia de modelo externo en judge | Media | Judge delega en REQ-10.2; si no hay modelo, retornar error |
| Scan lento en DB grande | Media | Scan tiene límite de observaciones; se puede filtrar por proyecto |
| memory_relations puede no tener datos de conflictos | Alta | GET /conflicts retorna array vacío si no hay, no error |

## Testing

- **List conflicts:** GET /conflicts → array (posiblemente vacío)
- **Judge:** POST /conflicts/judge → judgments array
- **Compare:** POST /conflicts/compare → similarity score
- **Get by ID:** GET /conflicts/{id} → 200 o 404
- **Stats:** GET /conflicts/stats → métricas
- **Scan:** POST /conflicts/scan → conflicts_found
- **Deferred:** GET /conflicts/deferred → array
- **Replay:** POST /conflicts/deferred/replay → resultados
