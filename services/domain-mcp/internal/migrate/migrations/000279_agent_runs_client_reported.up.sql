-- migration: 000279_agent_runs_client_reported
-- author: nunezlagos
-- issue: DOMAINSERV-147
-- description: agent_runs solo podía describir lo que ejecuta el server
--   (internal/runner/agent). A medida que la ejecución agéntica se mueve al
--   cliente —el diseño de DOMAINSERV-133/141— la telemetría de costo se vacía
--   sola: no hay tokens, ni modelo, ni estado de esos runs, y NADA avisa. El
--   síntoma de la pérdida es indistinguible de "esta semana se usaron menos
--   agentes".
--
--   Se agregan las tres columnas sin las que un run reportado por el cliente no
--   se puede leer ni atribuir:
--
--   - source: 'server' | 'client'. Sin esto, un run reportado y uno ejecutado
--     acá son la misma fila y cualquier auditoría de cobertura miente. DEFAULT
--     'server' para que las filas existentes queden correctamente clasificadas
--     sin backfill: hasta hoy TODAS las ejecutó el server.
--   - project_id: la dimensión por la que se consulta el costo. Las tools MCP
--     scopean por project explícito (el aislamiento por org se retiró en
--     000142/000143, así que el project ES el scope real, no un adorno sobre
--     RLS). ON DELETE SET NULL: borrar un project no debe borrar el histórico
--     de costo, que es contable.
--   - model: el modelo lo elige el CLIENTE en cada ejecución y puede no ser el
--     de agents.model (DOMAINSERV-135 acota modelo/effort por agente, pero el
--     override existe). Derivarlo del agente al leer daría un costo atribuido a
--     un modelo que no se usó. NULL en las filas viejas = sin dato, no cero.
--
--   Los tokens, el costo y el estado NO se agregan: ya existen desde 000015
--   (tokens_input, tokens_output, cost_usd, status) y el CHECK de status ya
--   restringe a los cinco estados válidos.
--
--   Un run reportado a medias no puede quedar colgado en 'running' porque el
--   cliente NO puede abrir un run: la tool de reporte acepta únicamente estados
--   terminales (ver internal/mcp/server/agent_run_report.go). Lo que se pierde
--   si el cliente muere a mitad es el reporte entero — ausencia de dato, no una
--   fila zombi. Es el mismo criterio de 000277: la ausencia de reporte significa
--   "sin dato", nunca otra cosa.
--
--   Single-tenant: sin organization_id, mismo criterio que 000180, 000277 y
--   000278.
-- breaking: no (aditiva, sin backfill; el DEFAULT clasifica lo existente)
-- estimated_duration: <1s (ADD COLUMN con DEFAULT no reescribe la tabla en pg11+)

ALTER TABLE agent_runs
  ADD COLUMN IF NOT EXISTS source VARCHAR(20) NOT NULL DEFAULT 'server'
    CHECK (source IN ('server', 'client')),
  ADD COLUMN IF NOT EXISTS project_id UUID REFERENCES projects(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS model VARCHAR(100);

-- Sin CONCURRENTLY a propósito: golang-migrate manda el archivo entero en un
-- Exec de simple protocol, o sea un implicit transaction block, y CONCURRENTLY
-- ahí devuelve 25001 (mismo motivo documentado en 000182, 000277 y 000278).
-- Innecesario además: project_id nace en ESTE archivo, así que el índice parcial
-- arranca vacío y el build es instantáneo.
CREATE INDEX IF NOT EXISTS agent_runs_project_source_idx
  ON agent_runs (project_id, source, created_at DESC)
  WHERE project_id IS NOT NULL;
