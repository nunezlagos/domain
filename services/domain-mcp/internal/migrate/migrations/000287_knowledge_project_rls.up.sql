-- migration: 000287_knowledge_project_rls
-- author: nunezlagos
-- issue: DOMAINSERV-185
-- description: RLS por project_id en knowledge_docs y knowledge_chunks, la segunda capa que
--   DOMAINSERV-182 dejó pendiente cuando cortó el leak cross-project con un solo WHERE
-- breaking: no — pero rompe cualquier lector/escritor que NO setee app.current_project_id
--   (ver la lista de caminos verificados abajo; los tres que faltaban se cablearon en este
--   mismo commit)
-- estimated_duration: <1s (100 docs y 157 chunks en prod; ENABLE/FORCE y CREATE POLICY son
--   cambios de catálogo, no recorren la tabla)
--
-- POR QUÉ: DOMAINSERV-182 cortó el leak de domain_knowledge_search con un filtro explícito
-- por project_id. Es UNA capa —el WHERE de la query— y por eso no cumple el check 1 de la
-- policy security-review-domain-specific: "toda query nueva scopea por project_id explícito
-- ADEMÁS de RLS". Sin RLS, la próxima query que alguien escriba y se olvide del WHERE vuelve
-- a filtrar, y nada la detiene a nivel de base.
--
-- EL EJE ES project_id Y NO organization_id, decidido en DOMAINSERV-187: la 000142 borró
-- organization_id de todas las tablas y la 000143 dropeó organizations. No hay de dónde
-- derivar la org de las filas existentes, así que el backfill sería inventado y las filas en
-- NULL quedarían invisibles bajo RLS. project_id sobrevivió, es NOT NULL en knowledge_docs y
-- está poblado.
--
-- MEDIDO EN PROD ANTES DE APLICAR (2026-08-05, HEAD aae45a85):
--   knowledge_docs    relrowsecurity=false, 100 filas vivas, project_id NULL en 0
--   knowledge_chunks  relrowsecurity=false, 157 filas
--   rol del pool de la app: app_user, rolbypassrls=FALSE  ← sin esto el RLS sería decorativo
--   rol del pool de auth:   app_admin, rolbypassrls=true
-- Sin filas en project_id NULL no hay nada que quede invisible: no hace falta backfill.
--
-- POR QUÉ knowledge_chunks VA POR EXISTS Y NO POR COLUMNA: knowledge_chunks NO tiene
-- project_id (sus columnas son id, knowledge_doc_id, chunk_index, content, content_tsv,
-- embedding, created_at, updated_at, status). Agregarle la columna sería desnormalizar y
-- abrir la puerta a que un chunk quede con un proyecto distinto al de su doc; la policy
-- deriva el proyecto del doc padre, que es la única fuente de verdad.
--
-- LOS CAMINOS QUE ESCRIBEN O LEEN ESTAS TABLAS, verificados uno por uno — esto es lo que
-- decide si la migración rompe prod, así que va acá y no en el commit:
--   internal/service/knowledge/*            las 4 tools domain_knowledge_* pasan por
--                                           rlsProyecto (server.go:253-257), con el guard
--                                           knowledge_rls_test.go que falla si alguna se
--                                           registra pelada
--   project_index_submit                    ya llamaba setProjectScope
--   orchestrator/analysis/service.go        ya seteaba el GUC por su cuenta
--   mcp/server/attachment_index.go          NO lo hacía — cableado en este commit
--   service/projectmerge                    mueve docs CROSS-project, así que ningún GUC
--                                           único puede ver el origen y escribir el destino:
--                                           pasa al pool de app_admin (BYPASSRLS) en este
--                                           commit, por decisión del usuario
--   internal/agentprotocol/protocol.go      solo menciona las tablas en texto de
--                                           documentación, no ejecuta SQL
--
-- current_project_id() replica la forma de current_org_id() (000274): nullif + EXCEPTION →
-- NULL. El nullif es lo que importa: sin él, current_setting devuelve '' cuando el GUC no
-- está seteado y el ::uuid revienta con un ERROR en vez de dar NULL — la búsqueda fallaría
-- ruidosamente en lugar de devolver cero filas, que es el contrato que el test de sabotaje
-- verifica.

CREATE OR REPLACE FUNCTION current_project_id() RETURNS UUID AS $$
BEGIN
  RETURN nullif(current_setting('app.current_project_id', true), '')::uuid;
EXCEPTION WHEN OTHERS THEN
  RETURN NULL;
END;
$$ LANGUAGE plpgsql STABLE;

ALTER TABLE knowledge_docs ENABLE ROW LEVEL SECURITY;
ALTER TABLE knowledge_docs FORCE ROW LEVEL SECURITY;
CREATE POLICY knowledge_docs_project_isolation ON knowledge_docs
  FOR ALL TO PUBLIC
  USING (project_id = current_project_id())
  WITH CHECK (project_id = current_project_id());

ALTER TABLE knowledge_chunks ENABLE ROW LEVEL SECURITY;
ALTER TABLE knowledge_chunks FORCE ROW LEVEL SECURITY;
CREATE POLICY knowledge_chunks_project_isolation ON knowledge_chunks
  FOR ALL TO PUBLIC
  USING (EXISTS (
    SELECT 1 FROM knowledge_docs d
    WHERE d.id = knowledge_chunks.knowledge_doc_id
      AND d.project_id = current_project_id()
  ))
  WITH CHECK (EXISTS (
    SELECT 1 FROM knowledge_docs d
    WHERE d.id = knowledge_chunks.knowledge_doc_id
      AND d.project_id = current_project_id()
  ));
