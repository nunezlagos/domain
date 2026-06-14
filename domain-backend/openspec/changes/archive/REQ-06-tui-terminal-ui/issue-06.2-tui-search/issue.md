# issue-06.2-tui-search

**Origen:** `REQ-06-tui-terminal-ui`
**Prioridad:** alta
**Tipo:** feature

## Historia de usuario

**Como** usuario de memoria
**Quiero** buscar observaciones con FTS5 desde el TUI presionando '/'
**Para** encontrar rápidamente información relevante sin salir de la terminal

## Criterios de aceptación

```gherkin
Scenario: '/' abre el input de búsqueda desde cualquier vista
  Given el TUI está en cualquier vista (dashboard, observations, sessions)
  When el usuario presiona '/'
  Then aparece un campo de texto en la parte inferior con el placeholder "Search..."

Scenario: Escribir texto y Enter ejecuta la búsqueda FTS5
  Given el input de búsqueda está visible
  When el usuario escribe "authentication" y presiona Enter
  Then se ejecuta una búsqueda FTS5 en la base de datos
  And se muestran los resultados en una lista

Scenario: Resultados de búsqueda muestran snippet + metadata
  Given hay resultados para "authentication"
  Then cada resultado muestra: título, snippet con match resaltado, proyecto, tipo, fecha
  And los resultados están ordenados por relevancia (rowid descendente por defecto)

Scenario: Navegación entre resultados con j/k
  Given hay múltiples resultados de búsqueda
  When el usuario presiona 'j'
  Then la selección baja al siguiente resultado
  When el usuario presiona 'k'
  Then la selección sube al resultado anterior

Scenario: Enter en un resultado abre la vista de detalle
  Given hay resultados de búsqueda visibles
  When el usuario selecciona un resultado y presiona Enter
  Then navega a la vista de detalle de esa observación

Scenario: ESC cierra la búsqueda y vuelve a la vista anterior
  Given el input de búsqueda está visible
  When el usuario presiona ESC
  Then el input desaparece
  And se vuelve a la vista anterior

Scenario: Búsqueda sin resultados muestra mensaje
  Given el usuario busca "xyz123nonexistent"
  When no hay resultados
  Then se muestra "No results found for 'xyz123nonexistent'"

Scenario: Búsqueda vacía no se ejecuta
  Given el input de búsqueda está visible
  When el usuario presiona Enter sin escribir nada
  Then no se ejecuta ninguna consulta
  And el input permanece visible
```

## Análisis breve

- **Qué pide realmente:** Componente de búsqueda con overlay de input, consulta FTS5 al store, resultados en lista, navegación, transición a detalle.
- **Módulos sospechados:** `internal/tui/search.go`, `internal/tui/search_model.go`
- **Riesgos / dependencias:** Depende de `store.SearchFTS5(query, opts)`. Requiere que FTS5 esté funcionando (issue-01.3). Snippet rendering con highlights.
- **Esfuerzo tentativo:** M

## Verificación previa

- [x] Revisar codebase (grep) — greenfield
- [ ] Revisar memorias engram (domain_mem_search)
- [ ] Revisar git log
- [ ] Probar en ambiente correcto
- [ ] Reproducir con perfil correcto
- [ ] Verificar caché / build
- [ ] Verificar feature flag / config

### Resultado de verificación

- **Estado:** greenfield
- **Evidencia:** No existe código Go en el proyecto
- **Acción derivada:** Implementar searchModel como submódulo del mainModel
