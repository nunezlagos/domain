-- migration: 000286_backfill_workflow_id_centinela
-- author: nunezlagos
-- issue: DOMAINSERV-229
-- description: reemplaza por NULL el centinela uuid-en-ceros que cuatro writers de
--   observabilidad persistieron en workflow_id antes de que usaran el guard workflowIDForRow
-- breaking: yes — no rompe schema ni código, pero los datos NO son reversibles (ver el .down)
-- estimated_duration: segundos (38.654 filas actualizadas sobre tres tablas; la más grande es
--   http_request_log con 33.888 de 158.741, y su índice parcial se actualiza en el mismo paso)
--
-- POR QUÉ: hasta el fix de DOMAINSERV-229, cuatro writers convertían el workflow del contexto
-- con wfID.String() en vez del guard. uuid.Nil.String() devuelve "00000000-...-000000000000",
-- que NO es cadena vacía, así que el NULLIF($n,'') de cada INSERT nunca disparaba y el
-- centinela quedó persistido en columnas text NULL. El fix corta las filas nuevas; no repara
-- las viejas. Mismo reparto de responsabilidades que 000284 tuvo con workflows.
--
-- MEDIDO EN PROD (2026-08-05, post-deploy de bd93f525), y esto es lo que el ticket había
-- dejado explícitamente sin medir porque el psql quedó bloqueado:
--   http_request_log       33.888 con el centinela,   365 NULL, 158.741 filas
--   sql_slow_queries        1.708 con el centinela,     7 NULL,   4.475 filas
--   mcp_tool_invocations    3.058 con el centinela,             33.575 filas
--   function_calls y error_events   0 y 0 — están vacías, así que no entran acá
--
-- QUÉ GANA, dimensionado y no exagerado: http_request_log tiene el índice parcial
-- idx_http_request_log_workflow con WHERE workflow_id IS NOT NULL, y hoy indexa 158.376
-- filas. De esas, 33.888 son el centinela y 124.488 son workflows reales (medido corriendo
-- este mismo UPDATE dentro de un BEGIN/ROLLBACK contra prod). O sea que el backfill saca el
-- 21% del índice, no la mayoría: es una mejora real pero acotada, y la afirmación de que "el
-- índice está lleno de basura" sería falsa.
--
-- La razón de peso es la otra, y aplica a las tres tablas por igual: un centinela
-- indistinguible de un dato real hace imposible diferenciar "esta corrida no tenía workflow"
-- de "el workflow es el uuid cero". Ese es el defecto que DOMAINSERV-229 vino a cerrar, y en
-- sql_slow_queries y mcp_tool_invocations —donde no hay índice sobre la columna— es la única
-- ganancia, porque alcanza.
--
-- POR QUÉ EL DML ESTÁ PERMITIDO ACÁ: la policy data-migration-methodology prohíbe DELETE y
-- UPDATE en .up.sql sobre tablas de USUARIO —projects, knowledge_observations, tickets,
-- issues, sdd_requirements, issue_*, prompts—. Estas tres son tablas de observabilidad
-- generadas por el propio server, ninguna está en esa lista, y el mismo criterio gobernó los
-- cuatro UPDATE de 000284 sobre workflows.
--
-- AUTORIZACIÓN: la irreversibilidad de los datos se le planteó al usuario con las cuatro
-- opciones (backfill completo, solo mcp_tool_invocations, cerrar sin backfill, o postergar) y
-- eligió el backfill completo el 2026-08-05. El pg_dump pre-migración lo corre install.sh y
-- aborta el deploy si falla (DOMAINSERV-26), así que hay de dónde volver si algo sale mal.
--
-- El literal va comparado como texto y no casteado a uuid a propósito: la columna es text, y
-- un ::uuid acá haría fallar la migración ante cualquier fila con un valor no parseable.

UPDATE http_request_log
SET workflow_id = NULL
WHERE workflow_id = '00000000-0000-0000-0000-000000000000';

UPDATE sql_slow_queries
SET workflow_id = NULL
WHERE workflow_id = '00000000-0000-0000-0000-000000000000';

UPDATE mcp_tool_invocations
SET workflow_id = NULL
WHERE workflow_id = '00000000-0000-0000-0000-000000000000';
