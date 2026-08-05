-- migration: 000284_backfill_workflows_lifecycle
-- author: nunezlagos
-- issue: DOMAINSERV-230
-- description: repara el histórico de workflows (ended_at anterior a started_at y estados
--   terminales que nunca se escribieron) y declara el invariante ended_at >= started_at
--   como NOT VALID, sin validarlo todavía
-- breaking: no
-- estimated_duration: segundos (23.222 filas, tres UPDATE sin reescritura de tabla; el
--   ADD CONSTRAINT NOT VALID no recorre la tabla)
--
-- POR QUÉ: hasta 000283 + el código de DOMAINSERV-229/230, nadie escribía un estado
-- terminal en workflows y started_at salía de un reloj distinto que ended_at. Medido en
-- prod: 23.221 de 23.222 filas en 'abandoned', CERO 'completed', y ended_at < started_at
-- en 19.260. El fix del código evita las filas nuevas; no repara las viejas.
--
-- LA FUENTE DE VERDAD ES flow_runs, no un now(). El workflow_id ES el flow_run_id
-- (DOMAINSERV-212), así que para toda fila con corrida hay dos timestamps autoritativos:
-- flow_runs.started_at y flow_runs.finished_at. Un now() inventaría una duración que
-- nadie midió, y por eso está excluido explícitamente de esta migración.
--
-- DESVIACIÓN DEL PLAN ORIGINAL, y el motivo: el plan decía "reparar ended_at desde
-- flow_runs.finished_at con GREATEST(ended_at, started_at) como fallback". No alcanza.
-- El workflow 79838c16 tiene f.finished_at 2026-08-02 20:32:25 contra w.started_at
-- 2026-08-03 02:00:33, o sea que finished_at es ANTERIOR al started_at de la fila:
-- copiarlo dejaría el invariante violado y el VALIDATE posterior reventaría. La causa es
-- que el started_at de esas filas es el bogus (lo puso el now() de Postgres cuando llegó
-- un touch tardío), no el ended_at. Así que se repara PRIMERO started_at desde
-- flow_runs.started_at, y después ended_at.

-- 1. started_at: el de la corrida manda. Es el único que no salió de un reloj equivocado.
UPDATE workflows w
SET started_at = f.started_at
FROM flow_runs f
WHERE f.id = w.id
  AND f.started_at IS NOT NULL
  AND f.started_at < w.started_at;

-- 2. ended_at: el finished_at de la corrida, si lo hay. GREATEST contra el started_at ya
--    corregido, porque una corrida puede tener finished_at nulo o anterior por el mismo
--    desfase de relojes que este ticket describe.
UPDATE workflows w
SET ended_at = GREATEST(f.finished_at, w.started_at)
FROM flow_runs f
WHERE f.id = w.id
  AND f.finished_at IS NOT NULL
  AND (w.ended_at IS NULL OR w.ended_at < w.started_at);

-- 3. Las filas sin corrida (o con corrida sin finished_at) que siguen violando el
--    invariante: se colapsa la duración a cero. Perder la duración es preferible a
--    persistir una negativa, y no hay otra fuente de la que reconstruirla.
UPDATE workflows
SET ended_at = started_at
WHERE ended_at IS NOT NULL
  AND ended_at < started_at;

-- 4. Reclasificación: las filas cuya corrida ya está en estado terminal pero que quedaron
--    en 'abandoned' o 'running' porque nadie escribía el cierre. El mapeo es IDENTIDAD
--    desde 000283, así que se copia sin traducir.
UPDATE workflows w
SET status = f.status
FROM flow_runs f
WHERE f.id = w.id
  AND f.status IN ('completed', 'failed', 'cancelled')
  AND w.status IN ('running', 'abandoned');

-- 5. El invariante, declarado y NO validado. NOT VALID porque VALIDATE recorre la tabla y
--    revienta el migrate si quedó una sola fila violándolo: el orden correcto es
--    confirmar el backfill contra prod y recién entonces validar en una migración aparte.
--    ended_at NULL está permitido a propósito: es una corrida viva.
ALTER TABLE workflows
  ADD CONSTRAINT workflows_ended_after_started
  CHECK (ended_at IS NULL OR ended_at >= started_at) NOT VALID;
