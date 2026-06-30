---
name: memory-graph-inference
description: >-
  Inferir y crear aristas tipadas entre memorias (knowledge graph) de un proyecto
  Domain. Usar cuando hay un IDE/agente conectado al MCP de Domain y se quiere
  enriquecer el grafo de memoria: detectar qué memorias se relacionan (se
  reemplazan, se contradicen, dependen unas de otras) y registrarlo.
trigger: >-
  Cuando el usuario pida "conectar memorias", "inferir relaciones", "armar el
  grafo de memoria", "relacionar observations/decisiones", o trabaje sobre el
  knowledge graph de un proyecto y haya un IDE conectado.
---

# Skill: Inferencia de aristas tipadas en el grafo de memoria

Este skill orienta a un subagente (Claude Code / OpenCode con el MCP de Domain
conectado) para enriquecer el grafo de memoria de un proyecto: descubrir
relaciones entre memorias y registrarlas como aristas tipadas.

## Concepto

Las memorias (observations) de un proyecto se relacionan implícitamente por
`project_id`, `session_id`, tags y solapamiento de texto. El grafo de memoria
las hace EXPLÍCITAS con aristas dirigidas y tipadas (`source -> target`):

| edge_type      | Significado (dirección source → target)                       |
| -------------- | ------------------------------------------------------------- |
| `supersedes`   | source reemplaza/revierte a target (target queda obsoleto)   |
| `contradicts`  | source contradice a target                                   |
| `derived_from` | source se deriva de target                                   |
| `depends_on`   | source depende de target                                     |
| `relates_to`   | relación genérica relevante                                  |

La **dirección importa**. Antes de crear una arista, decidí cuál memoria es la
`source` y cuál la `target` según la semántica de la tabla de arriba.

## Flujo recomendado (con IDE conectado)

Hay DOS caminos. Elegí según haya o no `MINIMAX_API_KEY` en el servidor.

### Camino A — Razonás vos (el subagente del IDE)

Preferido cuando querés control fino o cuando el server NO tiene MiniMax.

1. **Buscá / acotá las memorias del proyecto.**
   - `domain_mem_search(query, ...)` para encontrar el área de interés, o
   - elegí una observation ancla concreta (su `observation_id`).

2. **Pedí los pares candidatos** (sin LLM, sin embeddings — siempre disponible):

   ```
   domain_mem_suggest_links(
     project_slug = "<slug>",
     observation_id = "<uuid opcional para anclar>",
     max_pairs = 30
   )
   ```

   Devuelve `pairs[]`, cada uno con:
   - `source_id`, `target_id`
   - `source_content`, `target_content` (recortados)
   - `source_tags`, `target_tags`, `source_type`, `target_type`
   - señales: `same_session`, `shared_tags`, `lexical_overlap`, `signal_score`

3. **Razoná cada par.** Para cada candidato decidí:
   - ¿Hay realmente una relación que valga la pena registrar? Si no, descartalo.
   - ¿De qué TIPO es? (tabla de arriba)
   - ¿En qué DIRECCIÓN va? Ajustá quién es source y quién target.
   - Sé conservador: ante la duda, NO crees la arista.

4. **Creá las aristas** que decidiste, una por una:

   ```
   domain_mem_link(
     source_id = "<uuid>",
     target_id = "<uuid>",
     edge_type = "supersedes" | "contradicts" | "derived_from" | "depends_on" | "relates_to",
     note = "razón breve de la relación"
   )
   ```

   `domain_mem_link` es idempotente: si la arista activa ya existe, devuelve un
   error claro y no duplica. Seguí con el siguiente par.

### Camino B — Razona el servidor (MiniMax-M3)

Cuando el server tiene `MINIMAX_API_KEY` y querés delegar el razonamiento.

```
domain_mem_infer_edges_llm(
  project_slug = "<slug>",
  observation_id = "<uuid opcional>",
  max_pairs = 30
)
```

Internamente arma los mismos pares candidatos que `suggest_links`, le pide a
MiniMax-M3 que clasifique cada par y crea las aristas con `origin='inferred'`.
Devuelve `{candidates, created, skipped, existing, edges[]}`.

**Degradación:** si `MINIMAX_API_KEY` NO está seteada, el tool devuelve el error
`inferencia LLM requiere MINIMAX_API_KEY` SIN romper nada. En ese caso volvé al
**Camino A** (`suggest_links` + tu propio razonamiento + `domain_mem_link`).

## Verificá el resultado

Después de crear aristas, inspeccioná el grafo:

- `domain_mem_related(observation_id, direction = "both")` — vecinos de una memoria.
- `domain_mem_graph(project_slug)` — subgrafo del proyecto con conteo por tipo.
- `domain_mem_path(from_id, to_id)` — camino entre dos memorias.

## Reglas

- **No inventes IDs.** Usá solo los `source_id`/`target_id` que devuelve
  `suggest_links`.
- **Conservador antes que exhaustivo.** Una arista incorrecta contamina el grafo;
  preferí omitir a forzar.
- **La dirección es semántica**, no el orden en que vinieron en el par.
- **Single-tenant:** todo se aísla por `project_slug`. No hay organizaciones.
- `relates_to` es el tipo por defecto cuando hay relación pero no encaja en los
  otros. No lo uses para "todo con todo".
