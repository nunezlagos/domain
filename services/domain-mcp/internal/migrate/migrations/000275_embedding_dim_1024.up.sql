-- migration: 000275_embedding_dim_1024
-- author: nunezlagos
-- issue: DOMAINSERV-80
-- description: lleva TODAS las columnas pgvector de 1536 a 1024 para habilitar
--   embeddings reales con ollama/bge-m3 (1024 dim, multilingüe). Hasta ahora
--   DOMAIN_EMBEDDING_PROVIDER estaba en noop, así que los 2043 embeddings de
--   knowledge_observations y los 114 de knowledge_chunks son vector CERO
--   (verificado por SQL en prod antes de migrar): placeholders del NopEmbedder,
--   no datos reales. Se descartan y los regenera `domain embed-backfill` desde
--   el texto, que sigue intacto. skills (29 filas) y chat_document_embeddings
--   (0 filas) no tienen ni un embedding.
--   El bloque recorre pg_attribute en vez de nombrar tablas: es idempotente
--   (solo toca columnas cuya atttypmod difiere del target, así que re-correrlo
--   no hace nada) y cubre columnas vector que se agreguen después. Cada índice
--   se recrea con su definición original capturada de pg_indexes, preservando
--   el parámetro lists de cada uno (100/100/50/100).
-- breaking: yes
-- duration: <1s (los vectores descartados son ceros; el reindex es sobre ~2200 filas)

DO $$
DECLARE
  target_dim CONSTANT int := 1024;
  col   record;
  idx   record;
  idxdefs text[];
  d     text;
BEGIN
  FOR col IN
    SELECT c.relname AS tbl, a.attname AS col, a.atttypmod AS dim
    FROM pg_attribute a
    JOIN pg_class c ON c.oid = a.attrelid
    JOIN pg_namespace n ON n.oid = c.relnamespace
    JOIN pg_type t ON t.oid = a.atttypid
    WHERE t.typname = 'vector'
      AND n.nspname = 'public'
      AND c.relkind = 'r'
      AND a.attnum > 0
      AND NOT a.attisdropped
      AND a.atttypmod <> target_dim
  LOOP
    idxdefs := ARRAY[]::text[];
    FOR idx IN
      SELECT indexname, indexdef FROM pg_indexes
      WHERE schemaname = 'public' AND tablename = col.tbl
        AND indexdef LIKE '%' || col.col || '%'
        AND (indexdef LIKE '%ivfflat%' OR indexdef LIKE '%hnsw%')
    LOOP
      idxdefs := array_append(idxdefs, idx.indexdef);
      EXECUTE format('DROP INDEX IF EXISTS public.%I', idx.indexname);
    END LOOP;

    -- USING NULL descarta el vector cero: no se puede castear entre
    -- dimensiones y no hay información que preservar
    EXECUTE format(
      'ALTER TABLE public.%I ALTER COLUMN %I TYPE vector(%s) USING NULL::vector(%s)',
      col.tbl, col.col, target_dim, target_dim);

    FOREACH d IN ARRAY idxdefs LOOP
      EXECUTE d;
    END LOOP;

    RAISE NOTICE 'embedding dim: %.% -> vector(%)', col.tbl, col.col, target_dim;
  END LOOP;
END $$;
