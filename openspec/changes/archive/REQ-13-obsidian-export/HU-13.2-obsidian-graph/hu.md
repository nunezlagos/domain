# HU-13.2-obsidian-graph

**Origen:** `REQ-13-obsidian-export`
**Prioridad:** media
**Tipo:** feature

## Historia de usuario

**Como** usuario de Obsidian
**Quiero** tener un archivo `graph.json` con todos los nodos (observaciones) y sus relaciones (conflicts_with, supersedes, related)
**Para** visualizar el grafo de conocimiento de mi memoria de desarrollo en Obsidian Graph View

**Como** usuario power
**Quiero** elegir el modo de configuración: `preserve` (mantener graph existente), `force` (regenerar desde cero), `skip` (no generar graph)
**Para** controlar cuándo se recalcula el grafo según mi flujo de trabajo

## Criterios de aceptación

```gherkin
Scenario: graph.json contiene todos los nodos
  Given hay 5 observaciones en la store
  When se genera el graph
  Then graph.json tiene 5 nodos
  And cada nodo tiene id, type, title, project, tags

Scenario: Nodos tienen estructura correcta
  Given una observación con type="fix", project="Domain", topic_key="auth"
  When se genera el graph
  Then el nodo correspondiente tiene:
    | campo    | valor              |
    |----------|--------------------|
    | id       | 42                 |
    | type     | "fix"              |
    | title    | "Bug en login"     |
    | project  | "Domain"          |
    | tags     | ["auth"]           |

Scenario: graph.json contiene todos los links de memory_relations
  Given hay 3 relaciones en memory_relations
  When se genera el graph
  Then graph.json tiene 3 links
  And cada link tiene source, target, relation, confidence

Scenario: Links tienen estructura correcta
  Given una relación entre obs 1 y obs 2 con relation="conflicts_with", confidence=0.85
  When se genera el graph
  Then el link correspondiente tiene:
    | campo      | valor              |
    |------------|--------------------|
    | source     | 1                  |
    | target     | 2                  |
    | relation   | "conflicts_with"   |
    | confidence | 0.85               |

Scenario: Tipos de relación se mapean correctamente
  Given relaciones de tipo "conflicts_with", "supersedes", "related"
  When se genera el graph
  Then cada link tiene su relation original del tipo correspondiente

Scenario: Modo preserve no sobreescribe graph.json existente
  Given graph.json existe con contenido previo
  When se genera graph con mode=preserve
  Then graph.json no se modifica

Scenario: Modo force sobreescribe graph.json
  Given graph.json existe con contenido previo
  When se genera graph con mode=force
  Then graph.json se sobreescribe con nuevo contenido

Scenario: Modo skip no genera graph.json
  Given no hay graph.json previo
  When se genera graph con mode=skip
  Then graph.json no se crea

Scenario: Obs sin relaciones aparecen igual como nodos
  Given una observación sin relaciones en memory_relations
  When se genera el graph
  Then aparece como nodo en graph.json
  And no tiene links asociados

Scenario: Obs soft-deleted no aparecen en el graph
  Given una observación soft-deleted
  When se genera el graph
  Then no aparece como nodo
  And ningún link la referencia

Scenario: Graph JSON es válido según schema Obsidian
  Given se genera graph.json
  When se valida el JSON
  Then tiene formato { nodes: [...], links: [...] }
  And es parseable por Obsidian Graph View
```

## Análisis breve

- **Qué pide realmente:** Generador de `graph.json` en formato Obsidian Graph View. Nodos = observaciones activas. Links = relaciones desde memory_relations (conflicts_with, supersedes, related, candidate). Modos de configuración para controlar regeneración.
- **Módulos sospechados:** `internal/obsidian/` — `graph.go` con `GenerateGraph`, `GraphNode`, `GraphLink`, `Graph` structs
- **Riesgos / dependencias:** Depende de observations CRUD y memory_relations (HU-10.x); graph puede ser grande si hay muchas observaciones; modo preserve evita regeneración costosa
- **Esfuerzo tentativo:** M

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
- **Evidencia:** —
- **Acción derivada:** —
