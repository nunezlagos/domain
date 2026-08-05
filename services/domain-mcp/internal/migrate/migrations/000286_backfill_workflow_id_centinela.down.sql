-- migration: 000286_backfill_workflow_id_centinela (down)
--
-- ESTE DOWN NO REVIERTE NADA, y no puede.
--
-- La up puso NULL donde había el centinela. Para deshacerlo habría que saber CUÁLES de las
-- filas con workflow_id NULL lo tenían de origen y cuáles quedaron así por la up, y esa
-- distinción no se guardó en ninguna parte: antes de correrla había 365 filas legítimamente
-- NULL en http_request_log y 7 en sql_slow_queries. Un down que escribiera el centinela en
-- todas las filas NULL se las inventaría a esas 372 y dejaría los datos PEOR que antes de la
-- migración, no mejor.
--
-- Por eso la up declara breaking:yes y la decisión pasó por confirmación explícita del
-- usuario. La única vuelta atrás real es el pg_dump que install.sh toma antes de migrar
-- (DOMAINSERV-26). Vale la pena decirlo acá porque quien corra este down va a estar buscando
-- exactamente esa respuesta.
--
-- No hay cambio de schema que deshacer: la up es puro DML sobre tres columnas que ya eran
-- text NULL antes y siguen siéndolo después. Así que este archivo existe para que la cadena
-- de migraciones sea corrible hacia abajo sin cortarse, y para dejar escrito el motivo.

SELECT 'DOMAINSERV-229: el backfill del centinela uuid-nil no es reversible; la fuente de '
       'verdad previa es el pg_dump pre-migración' AS nota;
