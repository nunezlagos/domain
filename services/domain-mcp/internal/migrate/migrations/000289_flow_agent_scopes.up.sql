-- migration: 000289_flow_agent_scopes
-- author: nunezlagos
-- issue: DOMAINSERV-218
-- description: scopes de edición vigentes por (flow_run_id, agent_id), para que el rechazo por
--   allowlists solapadas tenga contra qué comparar al emitir un token de flow
-- breaking: no — tabla nueva, nadie la lee todavía; el grant sigue emitiendo igual si está vacía
-- estimated_duration: <1s (CREATE TABLE + índices sobre tabla vacía)
--
-- POR QUÉ
--
-- flow.ValidarParticionDisjunta existe desde el incremento 1 con 6 tests verdes y CERO callers
-- en producción: detecta el primer par de allowlists cuyos scopes se contienen, pero nadie la
-- llama porque un token aislado no puede responder "qué scopes hay vigentes en este flow". Esta
-- tabla es esa respuesta. Sin ella el criterio 3 del ticket no se puede cumplir, y el guard
-- seguiría siendo letra muerta (policy guards-deben-ejecutarse).
--
-- LA CLAVE ES (flow_run_id, agent_id) Y EL WRITE ES UN UPSERT, NO UN INSERT. Esto no es
-- preferencia: el matcher de claude_hook.go:41 dispara el grant en orchestrate_phase_result,
-- flow_status, orchestrate_confirm y flow_cancel, no solo en orchestrate, así que CADA CIERRE DE
-- FASE RE-EMITE el token. Con una fila por emisión, la segunda emisión del agente A chocaría con
-- la fila anterior del propio A y el rechazo por solapamiento LO BLOQUEARÍA A SÍ MISMO — un bug
-- que no aparecería hasta la segunda fase de cualquier flow real.
--
-- agent_id ES TEXT Y NO UUID, y '' es un valor legítimo: es el hilo principal. El agent_id lo
-- acuña el runtime del cliente y su formato no es un contrato nuestro; tiparlo como UUID ataría
-- el schema a un detalle de Claude Code que puede cambiar sin aviso. El default '' hace que la
-- clave única cubra también al hilo principal sin necesitar un NULL, que en Postgres no colisiona
-- consigo mismo y dejaría entrar filas duplicadas por la puerta de atrás.
--
-- EL EJE DE RLS ES project_id Y NO organization_id, igual que en la 000287 y la 000288: el eje org
-- es DECORATIVO en esta instancia porque internal/auth/apikey/store.go:30 define un canonicalOrgID
-- fijo que se asigna a toda credencial, así que un RLS por org no aislaría nada.

CREATE TABLE IF NOT EXISTS flow_agent_scopes (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  flow_run_id UUID NOT NULL REFERENCES flow_runs(id) ON DELETE CASCADE,
  -- '' = hilo principal. Ver arriba por qué no es NULL ni UUID.
  agent_id TEXT NOT NULL DEFAULT '',
  project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
  allowed_paths TEXT[] NOT NULL DEFAULT '{}',
  expires_at TIMESTAMPTZ NOT NULL,
  -- liberación explícita: un cancel o un cierre de flow suelta el territorio sin esperar el TTL
  revoked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT flow_agent_scopes_run_agente_uniq UNIQUE (flow_run_id, agent_id)
);

-- CONCURRENTLY NO se usa y no es un olvido: golang-migrate manda el archivo entero en un solo
-- Exec, que pgx ejecuta como implicit transaction block, y CREATE INDEX CONCURRENTLY dentro de
-- una transacción falla con 25001. Precedentes del mismo override: 000161, 000272:13 y 000288.
-- La tabla nace vacía, así que el lock es instantáneo.
-- domain-lint-ignore-next: require-concurrent-index
CREATE INDEX IF NOT EXISTS flow_agent_scopes_vigentes_idx
  ON flow_agent_scopes (flow_run_id, expires_at)
  WHERE revoked_at IS NULL;

-- current_project_id() con CREATE OR REPLACE: la 000287 ya la crea, y repetirla acá hace que esta
-- migración no dependa del orden ni de que un down previo la haya dejado sin función. nullif es lo
-- que importa: sin él current_setting devuelve '' cuando el GUC no está seteado y el ::uuid
-- revienta con ERROR en vez de dar NULL, así que la query fallaría ruidosamente en lugar de
-- devolver cero filas, que es el contrato que el RLS necesita.
CREATE OR REPLACE FUNCTION current_project_id() RETURNS UUID AS $$
BEGIN
  RETURN nullif(current_setting('app.current_project_id', true), '')::uuid;
EXCEPTION WHEN OTHERS THEN
  RETURN NULL;
END;
$$ LANGUAGE plpgsql STABLE;

ALTER TABLE flow_agent_scopes ENABLE ROW LEVEL SECURITY;
ALTER TABLE flow_agent_scopes FORCE ROW LEVEL SECURITY;
CREATE POLICY flow_agent_scopes_project_isolation ON flow_agent_scopes
  FOR ALL TO PUBLIC
  USING (project_id = current_project_id())
  WITH CHECK (project_id = current_project_id());
