-- migration: create_table_catalog (down)
-- author: mnunez@saargo.com
-- issue: REQ-42.1 (taxonomía y catálogo — source of truth)
-- description: elimina la tabla `table_catalog` (rollback). El catálogo
--   completo es derivable del seed en la migration up, por lo que el
--   down es un DROP puro. NO toca ninguna otra tabla del schema.
-- breaking: false
-- estimated_duration: <1s

BEGIN;

DROP TABLE IF EXISTS table_catalog;

COMMIT;
