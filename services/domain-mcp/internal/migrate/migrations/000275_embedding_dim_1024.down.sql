-- Reversa de 000275: vuelve las columnas pgvector a 1536.
-- Misma mecánica idempotente que el up. Los embeddings se descartan otra vez
-- (no son convertibles entre dimensiones); tras revertir hay que volver a
-- correr `domain embed-backfill` con un provider de 1536 (openai
-- text-embedding-3-small) para repoblarlos desde el texto.

DO $$
DECLARE
  target_dim CONSTANT int := 1536;
  col   record;
  idx   record;
  idxdefs text[];
  d     text;
BEGIN
  FOR col IN
    SELECT c.relname AS tbl, a.attname AS col
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

    EXECUTE format(
      'ALTER TABLE public.%I ALTER COLUMN %I TYPE vector(%s) USING NULL::vector(%s)',
      col.tbl, col.col, target_dim, target_dim);

    FOREACH d IN ARRAY idxdefs LOOP
      EXECUTE d;
    END LOOP;
  END LOOP;
END $$;
