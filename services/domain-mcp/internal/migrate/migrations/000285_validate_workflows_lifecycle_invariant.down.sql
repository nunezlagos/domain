-- migration: 000285_validate_workflows_lifecycle_invariant (down)
--
-- POSTGRES NO TIENE "UNVALIDATE CONSTRAINT", así que la reversión no puede ser una sola
-- sentencia simétrica a la up: hay que tirar el constraint y volver a declararlo NOT VALID.
--
-- El resultado es EXACTAMENTE el estado que dejaba 000284 — mismo nombre, mismo predicado,
-- convalidated=false — y por eso esta migración no es breaking. El predicado se repite
-- literal y NO se toca: si alguien lo cambia acá, un down seguido de un up dejaría la tabla
-- con un invariante distinto del que 000284 declaró, y la divergencia recién aparecería en un
-- ambiente levantado desde cero (policy migracion-aplicada-no-se-edita).
--
-- El DROP va con IF EXISTS porque este down tiene que ser corrible sobre una base donde el
-- constraint ya no esté (por ejemplo tras un down de 000284), sin abortar la cadena.
--
-- Lo que NO revierte, y es a propósito: los datos. El backfill de 000284 es irreversible por
-- las razones que su propio .down.sql explica; esta migración solo tocó metadata del
-- constraint, así que revertirla no restaura ni pretende restaurar ninguna fila.

ALTER TABLE workflows DROP CONSTRAINT IF EXISTS workflows_ended_after_started;

ALTER TABLE workflows
  ADD CONSTRAINT workflows_ended_after_started
  CHECK (ended_at IS NULL OR ended_at >= started_at) NOT VALID;
