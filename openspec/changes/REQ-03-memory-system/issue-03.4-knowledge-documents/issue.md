# issue-03.4-knowledge-documents

**Origen:** `REQ-03-memory-system`
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario
**Como** agente de IA
**Quiero** almacenar documentos largos con embeddings vectoriales (pgvector) y chunking automático
**Para** realizar búsqueda semántica por similitud coseno sobre conocimiento persistente (RAG)

## Criterios de aceptación

```gherkin
Feature: Knowledge Documents with Vector Search

  Background:
    Given Postgres tiene la extensión pgvector instalada
    And existe la tabla knowledge_documents

  Scenario: Almacenar un documento con chunking automático
    When guardo un documento de 5000 tokens con title "Arquitectura del sistema"
    Then el contenido se divide en chunks de ~512 tokens
    And cada chunk se almacena como un registro individual
    And todos los chunks comparten el mismo document_id
    And se genera embedding vectorial para cada chunk

  Scenario: Búsqueda semántica por similitud coseno
    Given existen chunks con embeddings de varios documentos
    When busco semánticamente con "¿cómo funciona el sistema de memoria?"
    Then obtengo chunks rankeados por similitud coseno (1 - cosine_distance)
    And los resultados incluyen el document_id, title, y fragmento de contenido

  Scenario: Búsqueda híbrida (vector + keyword)
    When busco con "sistema memoria" usando búsqueda híbrida
    Then combino ranking vectorial con tsvector rank
    And los resultados ponderan ambas métricas

  Scenario: Recuperar documento completo por document_id
    When consulto un documento por document_id
    Then obtengo todos sus chunks ordenados por chunk_index
    And puedo reconstruir el contenido completo concatenándolos

  Scenario: Filtrar por proyecto
    Given documentos de diferentes proyectos
    When filtro por project = "opencode-core"
    Then solo obtengo chunks de ese proyecto

  Scenario: Actualizar embedding de chunks existentes
    When actualizo el contenido de un chunk
    Then el embedding se recalcula
    And el chunk_index se mantiene
```

## Análisis breve

- **Qué pide realmente:** Tabla `knowledge_chunks` con vector embedding (pgvector), chunking strategy configurable, semantic search con cosine similarity, hybrid search (vector + keyword).
- **Módulos sospechados:** `internal/store/pg/knowledge.go`, `internal/rag/chunker.go`, `internal/rag/embedder.go`
- **Riesgos / dependencias:** Requiere extensión pgvector en Postgres. Depende del servicio de embeddings (REQ-06). Chunking requiere decisión de strategy (recursive, semantic, fixed-size).
- **Esfuerzo tentativo:** L

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
- **Evidencia:**
- **Acción derivada:**
