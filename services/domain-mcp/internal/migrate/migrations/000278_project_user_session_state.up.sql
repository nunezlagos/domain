-- migration: 000278_project_user_session_state
-- author: nunezlagos
-- issue: DOMAINSERV-188
-- description: el estado de arranque de sesión vivía en la tabla projects
--   (last_known_head, last_seen_branch, last_seen_cwd, agregadas en 000108), o
--   sea UNA fila por proyecto compartida por todos los usuarios. Con dos
--   personas trabajando el mismo proyecto se pisaban: A hacía bootstrap y
--   guardaba su HEAD, B abría sesión y recibía el de A como su last_known, así
--   que el `git log last_known..current` que el protocolo manda correr le
--   mostraba los commits de A como novedades propias. Ídem la rama y el cwd,
--   que la policy cross-project-context usa para resolver el proyecto.
--
--   Nada de eso fallaba con un error: cada usuario recibía un contexto de
--   arranque que describía la sesión de otro.
--
--   Lo que se separa acá es SOLO el puntero de "hasta dónde vi yo". El
--   CONTENIDO del proyecto —observaciones, policies, skills, tickets,
--   knowledge— sigue COMPARTIDO y no se toca: es conocimiento del equipo y un
--   usuario nuevo tiene que acceder a todo. La asimetría es deliberada.
--
--   No se reusa la tabla `sessions` (000007): es un histórico con
--   title/summary/started_at/ended_at, no un estado único por persona-proyecto,
--   y su organization_id referencia la tabla organizations que 000143 eliminó.
--
--   Aditiva a propósito: las columnas de projects quedan INTACTAS, solo dejan de
--   leerse para el bootstrap. Cero riesgo de perder datos y reversible. El
--   cleanup de esas columnas queda como deuda declarada, no escondida.
--
--   Single-tenant: sin organization_id, mismo criterio que 000180 y 000277 (el
--   aislamiento por org se retiró en 000142/000143). El aislamiento entre orgs
--   es DOMAINSERV-187 y este ticket NO lo aborda.
-- breaking: no (tabla nueva, sin backfill: el primer bootstrap de cada usuario
--   arranca con el puntero vacío, que es el comportamiento deseado — ver el
--   test UsuarioNuevoEnProyectoAjeno_ArrancaSinHead)
-- estimated_duration: <1s

CREATE TABLE IF NOT EXISTS project_user_session_state (
  user_id          UUID NOT NULL REFERENCES users(id)    ON DELETE CASCADE,
  project_id       UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
  last_known_head  VARCHAR(40),
  last_seen_branch VARCHAR(120),
  last_seen_cwd    VARCHAR(500),
  last_seen_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, project_id)
);

CREATE TRIGGER set_updated_at_project_user_session_state
  BEFORE UPDATE ON project_user_session_state
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Sin CONCURRENTLY a propósito: golang-migrate manda el archivo entero en un
-- Exec de simple protocol, o sea un implicit transaction block, y CONCURRENTLY
-- ahí devuelve 25001. Además la tabla nace vacía en este mismo archivo, así que
-- no hay nada que bloquear. La PK ya cubre el lookup por (user_id, project_id);
-- este índice es para la consulta inversa "quiénes tocaron este proyecto".
CREATE INDEX IF NOT EXISTS project_user_session_state_project_idx
  ON project_user_session_state (project_id, last_seen_at DESC);

GRANT SELECT, INSERT, UPDATE, DELETE ON project_user_session_state TO app_user;
GRANT ALL                            ON project_user_session_state TO app_admin;
