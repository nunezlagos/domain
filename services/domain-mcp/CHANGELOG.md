# Changelog

Todos los cambios notables de domain-mcp se documentan en este archivo.

El formato sigue [Keep a Changelog](https://keepachangelog.com/es-ES/1.0.0/).

## [Unreleased]

### Añadido

- **Memory graph** (mig 000175): grafo de relaciones explícitas y tipadas entre `knowledge_observations`, con aristas bi-temporales (valid_from/valid_to para valid time, created_at para transaction time). Tipos dirigidos: supersedes, derived_from, depends_on, contradicts, relates_to. Tabla `knowledge_observation_edges` + `observation/edge.go` + `memory_graph_tools.go`.
- **Code graph** (mig 000176): grafo de código del repo (Go v1) persistido en Postgres. Tablas `code_nodes`, `code_edges`, `code_index_files` con incrementalidad por content_hash + git_head. Paquete `internal/service/codegraph` + `code_graph_tools.go`.
- **Cruce memoria <-> código** (mig 000177): vínculo dirigido observation -> code_node (affects, decided_in, references, implements) que conecta el memory graph con el code graph. Tabla `knowledge_observation_code_links` + `codegraph/link.go`.
