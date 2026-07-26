-- migration: 000277_knowledge_observation_usage_log
-- author: nunezlagos
-- issue: DOMAINSERV-145
-- description: señal persistida de consumo de memoria, reportada por el CLIENTE.
--   Hoy no queda registro de si una observación inyectada en el prompt sirvió
--   de algo: el ranking puede ordenar por relevancia semántica, pero no por
--   utilidad demostrada.
--
--   Una fila por (turno, observación CANDIDATA) — o sea por cada observación
--   que el cliente dice haber tenido a la vista — con used=true en las que
--   declaró haber usado. Guardar los candidatos junto a los usados es lo que
--   da DENOMINADOR: sin él, "nunca reportada" y "nunca mostrada" son la misma
--   fila ausente y no hay tasa que medir. Es lo que hace la señal falsable.
--
--   La ausencia de reporte significa "sin dato", NUNCA "no sirvió". Si se
--   interpretara como negativo, toda observación poco reportada se degradaría
--   sola y el ranking se envenenaría.
--
--   project_id se denormaliza desde knowledge_observations para poder medir
--   atribución cruzada entre projects: SearchHybrid hoy NO filtra por project,
--   así que el mismatch es el estado normal y hay que MEDIRLO, no rechazarlo.
--
--   Append-only: nunca UPDATE, así que sin updated_at ni trigger. El UNIQUE
--   con ON CONFLICT DO NOTHING hace idempotente el reintento del turno.
--   Single-tenant: sin organization_id, mismo criterio que 000180 (el
--   aislamiento por org se retiró en 000142/000143).
--
--   NO se persiste la inyección desde el server: eso sería una escritura en el
--   camino de mem_search, que tiene una persona esperando.
-- breaking: no (tabla nueva, sin backfill)
-- estimated_duration: <1s

CREATE TABLE IF NOT EXISTS knowledge_observation_usage_log (
  id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  prompt_id      UUID NOT NULL REFERENCES prompt_captured(id)        ON DELETE CASCADE,
  observation_id UUID NOT NULL REFERENCES knowledge_observations(id) ON DELETE CASCADE,
  project_id     UUID REFERENCES projects(id) ON DELETE SET NULL,
  used           BOOLEAN NOT NULL DEFAULT false,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT knowledge_observation_usage_log_uniq UNIQUE (prompt_id, observation_id)
);

-- Sin CONCURRENTLY a propósito: golang-migrate manda el archivo entero en un
-- Exec de simple protocol, o sea un implicit transaction block, y CONCURRENTLY
-- ahí devuelve 25001. Además es innecesario — la tabla nace vacía en este mismo
-- archivo, así que el build es instantáneo y no hay nada que bloquear.
CREATE INDEX IF NOT EXISTS knowledge_observation_usage_log_obs_idx
  ON knowledge_observation_usage_log (observation_id, used, created_at DESC);

GRANT SELECT, INSERT, DELETE ON knowledge_observation_usage_log TO app_user;
GRANT ALL                     ON knowledge_observation_usage_log TO app_admin;
