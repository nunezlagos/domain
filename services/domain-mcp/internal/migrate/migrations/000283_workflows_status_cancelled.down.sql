-- migration: 000283_workflows_status_cancelled (down)
--
-- Restaura el CHECK a los 4 estados originales. NO es un no-op, pero SÍ puede fallar, y eso
-- es correcto: si ya existe alguna fila con status='cancelled', el ADD CONSTRAINT la rechaza
-- y el down aborta. Es lo deseable — bajar el constraint con filas que lo violan dejaría la
-- tabla en un estado que el schema dice imposible.
--
-- Si hay que bajar de verdad, primero hay que decidir a dónde van esas filas (probablemente
-- 'failed'), y esa es una decisión de negocio, no de la migración: colapsarlas
-- automáticamente acá borraría la distinción que este cambio vino a preservar.

ALTER TABLE workflows DROP CONSTRAINT IF EXISTS workflows_status_check;

ALTER TABLE workflows
  ADD CONSTRAINT workflows_status_check
  CHECK (status IN ('running', 'completed', 'failed', 'abandoned'));
