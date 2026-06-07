# HU-06.5-embedding-pgvector

**Origen:** `REQ-06-llm-embeddings`
**Persona:** dx-engineer, platform-engineer
**Prioridad tentativa:** alta
**Tipo:** feature

## Historia de usuario

**Como** desarrollador de la plataforma
**Quiero** providers de embeddings (OpenAI text-embedding-3-small, Anthropic, etc.) integrados con pgvector para almacenar y buscar vectores de embedding con cosine similarity
**Para** potenciar la búsqueda semántica en skills, observaciones y otros contenidos

## Criterios de aceptación

### Escenario 1: Generar embedding con OpenAI

```gherkin
Dado que DOMAIN_OPENAI_KEY está configurada
Cuando llamo a `openaiEmbedder.Embed(ctx, "Hola mundo")`
Entonces recibo un []float32 de longitud 1536
Y el vector está normalizado
```

### Escenario 2: Generar embedding con Anthropic

```gherkin
Dado que Anthropic soporta embeddings
Cuando llamo a `anthropicEmbedder.Embed(ctx, "Hola mundo")`
Entonces recibo un []float32 con longitud correcta
```

### Escenario 3: Embedding provider interface

```gherkin
Dado un embedder que implementa la interfaz:
  type Embedder interface {
      Name() string
      Embed(ctx context.Context, text string) ([]float32, error)
      EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
      Dimensions() int
  }
Cuando llamo a cualquiera de los métodos
Entonces funcionan según la interfaz
```

### Escenario 4: pgvector - CREATE EXTENSION vector

```gherkin
Dado que la migración 001 se ejecutó
Cuando verifico la base de datos
Entonces la extensión `vector` está habilitada
Y puedo crear columnas de tipo `vector(1536)`
```

### Escenario 5: pgvector - Insertar y buscar por cosine similarity

```gherkin
Dado que tengo una tabla `test_embeddings` con columna `embedding vector(1536)`
Y existen 3 filas con embeddings
Cuando ejecuto:
  SELECT id, 1 - (embedding <=> $embedding) AS similarity
  FROM test_embeddings
  ORDER BY similarity DESC
  LIMIT 5
Entonces recibo los resultados ordenados por similitud descendente
Y el score está entre 0 y 1
```

### Escenario 6: pgvector - Índice IVFFlat

```gherkin
Dado que la tabla skills tiene columna embedding vector(1536)
Cuando creo el índice:
  CREATE INDEX idx_skills_embedding ON skills
  USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100)
Entonces las búsquedas semánticas usan el índice
Y el plan de ejecución muestra "Index Scan" en lugar de "Seq Scan"
```

### Escenario 7: pgvector - Índice HNSW

```gherkin
Cuando creo el índice HNSW:
  CREATE INDEX idx_skills_embedding_hnsw ON skills
  USING hnsw (embedding vector_cosine_ops) WITH (m = 16, ef_construction = 200)
Entonces las búsquedas son más rápidas que con IVFFlat (tradeoff: más memoria)
```

### Escenario 8: Embedding batch

```gherkin
Cuando llamo a `embedder.EmbedBatch(ctx, []string{"texto1", "texto2", "texto3"})`
Entonces recibo [][]float32 con 3 vectores
Y cada vector tiene Dimensions() longitud
```

### Escenario 9: Embedding de texto vacío

```gherkin
Cuando llamo a `embedder.Embed(ctx, "")`
Entonces recibo un vector de zeros de longitud correcta
O error "empty text" dependiendo del provider
```

### Escenario 10: Integración con skills

```gherkin
Dado que un skill se crea con name="Resumir" y description="Resume textos"
Cuando se completa la creación
Entonces se genera un embedding del texto: name + " " + description
Y se almacena en la columna `embedding` de la tabla `skills`
```

## Análisis breve

- **Qué pide realmente:** Embedding providers (OpenAI, Anthropic) + integración pgvector. CREATE EXTENSION vector, columnas vector(1536), índices IVFFlat/HNSW, cosine similarity queries.
- **Módulos sospechados:** `internal/llm/embedding/`, `internal/database/migrations/`, `internal/database/queries/`
- **Riesgos / dependencias:** pgvector debe estar instalado en Postgres. La dimensión del embedding depende del modelo (1536 para text-embedding-3-small, 768 para others).
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
- **Evidencia:**
- **Acción derivada:**
