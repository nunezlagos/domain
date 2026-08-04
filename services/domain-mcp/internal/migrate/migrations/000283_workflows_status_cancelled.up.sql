-- migration: 000283_workflows_status_cancelled
-- author: nunezlagos
-- issue: DOMAINSERV-230
-- description: agrega 'cancelled' al CHECK de workflows.status para que el mapeo desde
--   flow_runs.status sea identidad y no traducción
-- breaking: no
-- estimated_duration: instantáneo (el CHECK se re-crea sin reescribir la tabla; 23.222 filas,
--   todas con status ya permitido por el constraint nuevo)
--
-- POR QUÉ: flow_runs tiene 'cancelled' y workflows no lo admitía, así que el cierre de un
-- flow cancelado tenía que colapsarlo en 'failed' o en 'abandoned'. Las dos opciones pierden
-- información que no se puede reconstruir: 'failed' borra la distinción entre "falló" y "lo
-- canceló un humano", y 'abandoned' pisa el significado que ese estado ya tiene acá (el idle
-- timeout del heartbeat), dejando inauditable la diferencia entre una corrida que alguien
-- detuvo y una que se murió sola. Decisión del usuario: los cancelados quedan cancelados.
--
-- ADVERTENCIA DE ORDEN: esta migración va ANTES del binario que escribe 'cancelled'. Al
-- revés, el INSERT/UPDATE revienta contra el constraint viejo. En un deploy el orden es el
-- natural (migrate corre antes de que el server arranque), pero no lo invierta quien aplique
-- esto a mano.
--
-- El CHECK original vive en 000210_create_workflows.up.sql:4 y NO se edita: está aplicada en
-- prod, y editarla haría que un deploy limpio construya un schema distinto al de producción
-- (policy migracion-aplicada-no-se-edita).

ALTER TABLE workflows DROP CONSTRAINT IF EXISTS workflows_status_check;

ALTER TABLE workflows
  ADD CONSTRAINT workflows_status_check
  CHECK (status IN ('running', 'completed', 'failed', 'abandoned', 'cancelled'));
