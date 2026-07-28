-- migration: 000278_project_user_session_state (down)
-- author: nunezlagos
-- issue: DOMAINSERV-188
-- description: revierte la tabla de estado de sesión por usuario. Reversible sin
--   pérdida de datos de usuario: las columnas equivalentes de projects quedaron
--   INTACTAS en el up —la migración fue aditiva a propósito— así que al bajar,
--   el bootstrap vuelve a leer de ahí y el peor caso es el bug original (dos
--   personas pisándose el puntero), no una pérdida.
--
--   Lo único que se pierde es el puntero por usuario acumulado desde el deploy.
--   Es telemetría de arranque, no fuente de verdad de nada: el efecto de
--   perderlo es que cada usuario se trata como primera vez, que es justamente el
--   comportamiento definido para un usuario nuevo.
-- breaking: no
-- estimated_duration: <1s

DROP TABLE IF EXISTS project_user_session_state CASCADE;
