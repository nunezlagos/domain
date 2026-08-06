-- migration: 000290_skills_slug_unico_por_proyecto (down)
-- author: nunezlagos
-- issue: DOMAINSERV-223
-- description: revierte el índice único de slug por proyecto
-- breaking: no
-- estimated_duration: <1s
--
-- El soft-delete del duplicado NO se revierte automáticamente: reactivarlo volvería a violar el
-- índice si alguien re-aplica la up, y sobre todo no se puede saber desde acá si en el ínterin
-- alguien editó la fila que quedó viva. Revertirlo es una decisión con contexto, no un rollback
-- mecánico. Para hacerlo a mano:
--
--   UPDATE skills SET deleted_at = NULL WHERE id = '<el uuid descartado>';

DROP INDEX IF EXISTS skills_slug_por_proyecto_uniq;
