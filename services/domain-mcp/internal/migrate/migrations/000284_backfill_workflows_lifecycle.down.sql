-- migration: 000284_backfill_workflows_lifecycle (down)
--
-- El down BAJA el constraint y NO revierte el backfill. No es pereza: es que no se puede.
--
-- Los cuatro UPDATE de la up sobrescriben started_at, ended_at y status con valores
-- derivados de flow_runs. Los valores ORIGINALES no se guardaron en ninguna parte, así
-- que no hay de dónde reconstruirlos: eran 19.260 filas con ended_at anterior a su propio
-- started_at, o sea duraciones negativas. Un down que "restaurara" algo tendría que
-- inventarlo, y eso es peor que no revertir.
--
-- Si alguna vez hace falta el histórico previo, la única fuente es un backup de la tabla
-- anterior a esta migración. Vale la pena decirlo acá porque quien corra este down va a
-- estar buscando exactamente esa respuesta.
--
-- Lo que SÍ se revierte es el constraint, que es lo único que cambia el schema y lo único
-- que puede bloquear un INSERT hacia adelante.

ALTER TABLE workflows DROP CONSTRAINT IF EXISTS workflows_ended_after_started;
