-- migration: 000290_skills_slug_unico_por_proyecto
-- author: nunezlagos
-- issue: DOMAINSERV-223
-- description: el slug de una skill de proyecto pasa a ser único por proyecto, y se descarta el
--   duplicado vivo que la ausencia de esa restricción dejó entrar
-- breaking: no — MEDIDO en prod: 1 solo grupo duplicado en toda la tabla, y el descarte es
--   soft-delete, así que la fila sigue ahí y se puede revertir con un UPDATE
-- estimated_duration: <1s (un UPDATE sobre una fila + un CREATE INDEX sobre ~cientos de filas)
--
-- POR QUÉ
--
-- El único que existe cubre SOLO las skills globales:
--
--   skills_slug_global_uniq ON skills(slug) WHERE project_id IS NULL AND deleted_at IS NULL
--
-- así que las de proyecto no tienen ninguna restricción de unicidad, ni global ni por proyecto.
-- No hay otro índice sobre slug (verificado con pg_indexes contra prod el 2026-08-06).
--
-- Y el camino de escritura no compensa esa ausencia: project_skill_register hace un INSERT INTO
-- skills SIN ON CONFLICT (project_skill_tools.go:108-115), así que cada re-registro con el mismo
-- slug crea una fila nueva. El INSERT en project_skills de la línea siguiente SÍ usa
-- ON CONFLICT DO NOTHING — el patrón se conocía; al de skills le faltaba el constraint contra el
-- cual hacer conflict. Esta migración se lo da.
--
-- EL DUPLICADO ES REAL Y TIENE IMPACTO OBSERVABLE. En prod hay dos filas activas con slug
-- 'correr-migraciones-manuales' en el proyecto ace-did-2025, creadas con 3 minutos de diferencia.
-- El hook de skills sugeridas las listó A LAS DOS en el mismo prompt, o sea que el duplicado ya
-- consume contexto en cada turno de ese proyecto.
--
-- EL ORDEN DE LOS DOS STATEMENTS NO ES ARBITRARIO: con las dos filas vivas, el CREATE UNIQUE
-- INDEX falla. La limpieza va primero. Es el mismo tropiezo que documentó la 000288.

-- 1. Descarte del duplicado. Se conserva el MÁS VIEJO de cada grupo, igual criterio que la 000288
--    con los slugs de webhooks. Acá además coincide con el de más contenido (3975 vs 2683 chars),
--    que es lo que uno querría conservar si tuviera que elegir a mano.
--
--    Es SOFT-DELETE: la fila queda en la tabla con deleted_at, así que revertir es un UPDATE y no
--    una restauración de backup.
UPDATE skills SET deleted_at = now()
WHERE project_id IS NOT NULL
  AND deleted_at IS NULL
  AND id NOT IN (
    SELECT DISTINCT ON (project_id, slug) id
    FROM skills
    WHERE project_id IS NOT NULL AND deleted_at IS NULL
    ORDER BY project_id, slug, created_at
  );

-- 2. La restricción que faltaba. Parcial por dos razones: excluye las globales, que ya tienen la
--    suya y colisionarían con esta, y excluye las borradas, para que un slug descartado se pueda
--    volver a usar — que es lo que hace viable el delete+create como forma de rehacer una skill.
--
-- CONCURRENTLY no se usa y no es un olvido: golang-migrate manda el archivo entero en un solo
-- Exec, que pgx ejecuta como implicit transaction block, y CREATE INDEX CONCURRENTLY dentro de
-- una transacción falla con 25001. Precedentes del mismo override: 000161, 000272:13, 000288.
-- domain-lint-ignore-next: require-concurrent-index
CREATE UNIQUE INDEX IF NOT EXISTS skills_slug_por_proyecto_uniq
  ON skills (project_id, slug)
  WHERE project_id IS NOT NULL AND deleted_at IS NULL;
