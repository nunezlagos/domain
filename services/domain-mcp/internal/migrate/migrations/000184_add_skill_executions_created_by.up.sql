-- migration: 000184_add_skill_executions_created_by
-- author: NunezLagos
-- issue: HU-52.2
-- description: deuda de HU-52.2 — skill_executions no tenia columna de caller, por
--   lo que unique_callers_count del aggregator quedaba clavado en 0 (TODO). Esta
--   migracion agrega created_by UUID (nullable) con FK a users(id) ON DELETE SET
--   NULL: el caller que origina la ejecucion (Principal del MCP/HTTP) se persiste
--   cuando existe; los triggers de sistema (cron, webhook) dejan NULL. Sin backfill
--   (las filas historicas quedan en NULL, son ejecuciones sin caller conocido). El
--   aggregator pasa a computar COUNT(DISTINCT created_by) FILTER (created_by IS NOT
--   NULL). Single-tenant (regla dura 1): NO se toca organization_id (dropeada en
--   000142); el ALTER solo agrega created_by.
-- breaking: no (columna nullable, sin backfill, FK NOT VALID + VALIDATE).
-- estimated_duration: unknown

-- created_by: usuario que origino la ejecucion. Nullable porque hay call-sites
-- sin user humano (cron de sistema, webhook externo). ON DELETE SET NULL: si se
-- borra el usuario, la ejecucion historica se conserva con caller desconocido.
ALTER TABLE skill_executions ADD COLUMN created_by UUID;

-- FK NOT VALID: skill_executions es tabla existente; NOT VALID evita el full table
-- scan + lock al agregar la constraint (require-not-valid-fk).
ALTER TABLE skill_executions
  ADD CONSTRAINT skill_executions_created_by_fkey
  FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL NOT VALID;

-- VALIDATE en statement separado: valida las filas existentes (todas NULL hoy)
-- sin el lock fuerte del ADD CONSTRAINT con validacion implicita.
ALTER TABLE skill_executions VALIDATE CONSTRAINT skill_executions_created_by_fkey;
